# eipgam

eipgam wires AI coding agents to the models and subscriptions you have: one local gateway serves them all, and when a quota runs out it moves on to your next account. Pick a provider and model per agent in the menu-bar app, the TUI (`eipgam tui`) or the CLI.

eipgam is based on Magpie. Main differences:

- **No telemetry.** No usage counts, install id, plugin likes or partner counts are sent.
- **Supports 22 agents:** Claude Code, Claude Desktop, Claude Science, Codex, Gemini CLI, Antigravity CLI, OpenCode, Pi, Cursor CLI, Cursor Private Inference, Zed, VS Code Chat, VS Code Insiders, VSCodium Chat, Copilot CLI, Devin, Kimi Code, Muse Code, Grok Build, Grok App, Raycast, T3 Code.
- **No mainland-China providers or China-only features.** The overseas services of Chinese vendors (Z.ai, Kimi, MiniMax, StepFun, Qwen intl, Alibaba Cloud, Qwen Cloud, SiliconFlow, Tencent Cloud intl, Xiaomi MiMo's Singapore and Europe plans) stay. China mirrors and daily check-ins are removed.

## Build and install (macOS)

Needs Go and Node.

```sh
make app VERSION=0.2.0
codesign --force --deep --sign - --identifier com.sichengchen.eipgam eipgam.app
ditto eipgam.app /Applications/eipgam.app
open /Applications/eipgam.app
```

eipgam runs in the menu bar. For the `eipgam` command, use Settings → Command line, or link `/Applications/eipgam.app/Contents/MacOS/eipgam` into your PATH.

The app checks [eipgam releases on GitHub](https://github.com/sichengchen/eipgam/releases) for updates. It verifies the SHA-256 and GitHub build attestation before installation. Automatic checks are controlled in Settings → About.

eipgam stores its configuration in `~/.config/eipgam` (or `$XDG_CONFIG_HOME/eipgam`).

## Using it

```sh
eipgam provider add openrouter sk-or-…           # a preset needs only the key
eipgam claude openrouter/anthropic/claude-sonnet-5-5
eipgam codex moonshot/kimi-k3                    # Codex on Kimi
eipgam group add "Opus anywhere" models=claude/claude-opus-5-5,copilot/claude-opus-5.5 routing=smart
eipgam claude group/opus-anywhere                # fails over between subscriptions
eipgam quota                                     # what's left on every plan
eipgam tui
```

See [the full reference](docs/reference.md) for commands and configuration.

## License

eipgam is licensed under the [MIT License](LICENSE). The original Magpie license and other dependency notices are in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
