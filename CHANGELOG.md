# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once the first release is
tagged. Until then, source builds report the version `dev`.

## [0.2.0](https://github.com/Gitlawb/zero/compare/v0.1.0...v0.2.0) (2026-07-02)


### Features

* **providers:** add `zero providers models` to discover a provider's models ([#386](https://github.com/Gitlawb/zero/issues/386)) ([0bc8074](https://github.com/Gitlawb/zero/commit/0bc8074c97b0310e4a9d70c3f967003ee5e8a59f))
* **providers:** add KiloCode and OpenCode provider support ([#388](https://github.com/Gitlawb/zero/issues/388)) ([b1ccb6d](https://github.com/Gitlawb/zero/commit/b1ccb6d9c1875377f5e5ea81a1304edd1e41ab4f))
* require manual approval before npm publish + drop release-as pin ([#369](https://github.com/Gitlawb/zero/issues/369)) ([bd89a1f](https://github.com/Gitlawb/zero/commit/bd89a1f451643c1b65ec803070abc7b116631ebe))


### Bug Fixes

* **config:** unbrick first-run setup — default google/anthropic models, enter setup on fixable config errors ([#385](https://github.com/Gitlawb/zero/issues/385)) ([72eed06](https://github.com/Gitlawb/zero/commit/72eed06b4f94c43d75d31fe54a58d2f566de059e))
* **config:** use ~/.config on macOS and enter setup when no provider ([#371](https://github.com/Gitlawb/zero/issues/371)) ([#372](https://github.com/Gitlawb/zero/issues/372)) ([027a8f2](https://github.com/Gitlawb/zero/commit/027a8f2768b17b89f5c8270887f156e2ccda69ea))
* **gemini:** strip unsupported JSON Schema fields from tool declarations ([#374](https://github.com/Gitlawb/zero/issues/374)) ([39e7100](https://github.com/Gitlawb/zero/commit/39e7100674150144a1152e3110c64c7cf0321d64)), closes [#373](https://github.com/Gitlawb/zero/issues/373)
* **tools:** CRLF line ending mismatch in edit_file tool on Windows ([#378](https://github.com/Gitlawb/zero/issues/378)) ([33dc7ae](https://github.com/Gitlawb/zero/commit/33dc7ae2cc82c5389675531e1416856dae7151ce))
* **tools:** require permission before web_search requests ([#382](https://github.com/Gitlawb/zero/issues/382)) ([960db96](https://github.com/Gitlawb/zero/commit/960db9660e4e31dc588fe8f7d6f116ff5e225566))

## 0.1.0 (2026-07-02)


### Features

* publish zero to npm via release-please ([#367](https://github.com/Gitlawb/zero/issues/367)) ([8eccc26](https://github.com/Gitlawb/zero/commit/8eccc2669887bc38d35bc16a315c888e4d9ec43a))
* **tui:** FILES sidebar panel with click-to-select and file drill-in ([#365](https://github.com/Gitlawb/zero/issues/365)) ([142c548](https://github.com/Gitlawb/zero/commit/142c548c89a8652ce300e64ddf1228ee36df7606))


### Bug Fixes

* **auth:** propagate credentials to every provider-build surface and pin children to the live provider ([#366](https://github.com/Gitlawb/zero/issues/366)) ([6e0a665](https://github.com/Gitlawb/zero/commit/6e0a665118fe0e09c4b07d482dd18f86045acd2b))

## [Unreleased]

### Added
- `SECURITY.md` with a private vulnerability-reporting path, `CODE_OF_CONDUCT.md`, this changelog, and
  GitHub issue/PR templates.
- Interactive `/theme` picker: bare `/theme` opens a popup that live-previews each palette as you move
  and applies on select (Esc reverts).
- Ten built-in color themes alongside the `dark`/`light` built-ins — `dracula`, `nord`, `gruvbox`,
  `tokyo-night`, `catppuccin`, `one-dark`, `solarized-dark`, `rose-pine`, `everforest`, and
  `solarized-light` — selectable via `/theme <name>`, `--theme <name>`, or `ZERO_THEME`. Every palette
  is contrast-audited to WCAG AA. The built-in light theme was reworked for legibility.
- `--theme <name>` flag for the TUI, accepting `auto` or any registered theme (previously only the
  `ZERO_THEME` env var existed).
- "Accessibility / Appearance" section in the README documenting `NO_COLOR`, `ZERO_THEME`, `/theme`,
  and `ZERO_NO_FADE`.

### Changed
- Provider connectivity health checks now allow loopback hosts for explicitly user-configured local
  providers (Ollama / LM Studio), so the keyless local-model path verifies instead of failing with
  "localhost hosts are blocked". The SSRF guard for fetched/remote URLs is unchanged.
- Auth (401/403) errors now show a curated, actionable message pointing at `zero auth` / setup; the
  raw upstream body is shown only under a verbose/debug flag.
- No-provider / missing-key errors now point at `zero setup` and `zero auth`, and distinguish a
  missing key from a rejected key.
- `zero doctor` no longer reports "Overall: pass" when no provider credential is configured, and
  formats the missing-language-server list for humans (no raw Go `map[...]`).
- Raised the `faint`/`faintest` theme tokens (and the light-theme accent) to meet WCAG AA contrast for
  the content they carry.
- `NO_COLOR` is now honored for any non-empty value, per the no-color.org spec.

### Removed
- The inert `/input-style` slash command (it had no backend).

### Fixed
- README/`go.mod` Go-version mismatch and other stale public-release docs claims.
