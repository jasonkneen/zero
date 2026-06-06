package sessions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// CheckpointsDir is the per-session subdirectory holding content-addressed blobs.
const CheckpointsDir = "checkpoints"

const defaultMaxCheckpointBytes = 5 << 20 // 5 MiB

// CheckpointFile records the before-mutation state of one workspace file.
type CheckpointFile struct {
	Path    string `json:"path"`
	Blob    string `json:"blob,omitempty"`    // sha256 of prior content, "" if absent/skipped
	Absent  bool   `json:"absent,omitempty"`  // file did not exist before (restore -> delete)
	Skipped bool   `json:"skipped,omitempty"` // exceeded size cap; not recoverable
	Bytes   int    `json:"bytes,omitempty"`
}

// CheckpointPayload is the payload of an EventSessionCheckpoint event. It indexes
// the before-state blobs captured for one mutating tool call.
type CheckpointPayload struct {
	Tool  string           `json:"tool"`
	Files []CheckpointFile `json:"files"`
}

// CheckpointsEnabled reports whether checkpoint capture is enabled (default on;
// disabled with ZERO_CHECKPOINTS=off).
func CheckpointsEnabled() bool {
	return os.Getenv("ZERO_CHECKPOINTS") != "off"
}

func maxCheckpointBytes() int {
	if raw := os.Getenv("ZERO_CHECKPOINT_MAX_BYTES"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxCheckpointBytes
}

func (store *Store) blobsDir(sessionID string) string {
	return filepath.Join(store.sessionPath(sessionID), CheckpointsDir, "blobs")
}

func (store *Store) blobPath(sessionID, hash string) string {
	return filepath.Join(store.blobsDir(sessionID), hash)
}

// CaptureToolCheckpoint snapshots the current (before-mutation) content of each
// path and records an EventSessionCheckpoint indexing the blobs. Capture is
// best-effort: an unreadable file is recorded as skipped rather than failing the
// caller. Returns the appended event (or a zero Event if there was nothing to do).
func (store *Store) CaptureToolCheckpoint(sessionID, workspaceRoot, tool string, paths []string) (Event, error) {
	payload, ok := store.SnapshotForCheckpoint(sessionID, workspaceRoot, tool, paths)
	if !ok {
		return Event{}, nil
	}
	return store.AppendEvent(sessionID, AppendEventInput{Type: EventSessionCheckpoint, Payload: payload})
}

// SnapshotForCheckpoint reads and stores the before-mutation blobs for paths and
// returns the checkpoint payload WITHOUT appending an event. Callers that batch
// session events (e.g. the TUI) snapshot here — before the mutation runs — then
// append the EventSessionCheckpoint themselves. Returns ok=false when there is
// nothing to record (disabled, no paths, or no capturable files).
func (store *Store) SnapshotForCheckpoint(sessionID, workspaceRoot, tool string, paths []string) (CheckpointPayload, bool) {
	if !CheckpointsEnabled() || len(paths) == 0 {
		return CheckpointPayload{}, false
	}
	capBytes := maxCheckpointBytes()
	files := make([]CheckpointFile, 0, len(paths))
	for _, rel := range paths {
		entry := CheckpointFile{Path: rel}
		abs := filepath.Join(workspaceRoot, rel)
		info, statErr := os.Stat(abs)
		if statErr != nil {
			// Treat anything we cannot stat as absent (new file) — restore deletes it.
			entry.Absent = true
			files = append(files, entry)
			continue
		}
		if info.IsDir() {
			continue
		}
		if int(info.Size()) > capBytes {
			entry.Skipped = true
			entry.Bytes = int(info.Size())
			files = append(files, entry)
			continue
		}
		content, readErr := os.ReadFile(abs)
		if readErr != nil {
			entry.Skipped = true
			files = append(files, entry)
			continue
		}
		hash, writeErr := store.writeBlob(sessionID, content)
		if writeErr != nil {
			entry.Skipped = true
			files = append(files, entry)
			continue
		}
		entry.Blob = hash
		entry.Bytes = len(content)
		files = append(files, entry)
	}
	if len(files) == 0 {
		return CheckpointPayload{}, false
	}
	return CheckpointPayload{Tool: tool, Files: files}, true
}

// writeBlob stores content under its sha256 (content-addressed, deduplicated) and
// returns the hex hash. An existing blob with the same hash is left untouched.
func (store *Store) writeBlob(sessionID string, content []byte) (string, error) {
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	dir := store.blobsDir(sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create checkpoint blob dir: %w", err)
	}
	path := store.blobPath(sessionID, hash)
	if _, err := os.Stat(path); err == nil {
		return hash, nil // dedup: identical content already stored
	}
	tmp := fmt.Sprintf("%s.tmp-%d", path, store.idCounter.Add(1))
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return "", fmt.Errorf("write checkpoint blob: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("commit checkpoint blob: %w", err)
	}
	return hash, nil
}

// readBlob returns the content stored under a hash, verifying that the content
// still hashes to the requested sha256. A mismatch (corruption/tampering) is
// returned as an error so the caller skips the path rather than writing
// untrusted content as truth.
func (store *Store) readBlob(sessionID, hash string) ([]byte, error) {
	content, err := os.ReadFile(store.blobPath(sessionID, hash))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	if got := hex.EncodeToString(sum[:]); got != hash {
		return nil, fmt.Errorf("checkpoint blob %s failed integrity check (got %s)", hash, got)
	}
	return content, nil
}

// pruneOrphanBlobs removes blobs not referenced by any checkpoint event (e.g. after
// a rewind discards later checkpoints). Best-effort; returns count removed.
func (store *Store) pruneOrphanBlobs(sessionID string) (int, error) {
	referenced, err := store.referencedBlobs(sessionID)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(store.blobsDir(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || referenced[e.Name()] {
			continue
		}
		if err := os.Remove(store.blobPath(sessionID, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

func (store *Store) referencedBlobs(sessionID string) (map[string]bool, error) {
	events, err := store.ReadEvents(sessionID)
	if err != nil {
		return nil, err
	}
	refs := map[string]bool{}
	for _, ev := range events {
		if ev.Type != EventSessionCheckpoint {
			continue
		}
		var payload CheckpointPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		for _, f := range payload.Files {
			if f.Blob != "" {
				refs[f.Blob] = true
			}
		}
	}
	return refs, nil
}

// sortedCheckpointsAfter returns checkpoint events with Sequence > targetSeq,
// newest first (so restoring applies the snapshot closest to the target last).
func (store *Store) sortedCheckpointsAfter(sessionID string, targetSeq int) ([]Event, error) {
	events, err := store.ReadEvents(sessionID)
	if err != nil {
		return nil, err
	}
	var checkpoints []Event
	for _, ev := range events {
		if ev.Type == EventSessionCheckpoint && ev.Sequence > targetSeq {
			checkpoints = append(checkpoints, ev)
		}
	}
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Sequence > checkpoints[j].Sequence
	})
	return checkpoints, nil
}
