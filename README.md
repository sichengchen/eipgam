# magpie (fork)

magpie wires AI coding agents to the models and subscriptions you have: one local gateway serves them all, and when a quota runs out it moves on to your next account. Pick a provider and model per agent in the menu-bar app, the TUI (`magpie tui`) or the CLI.

This is a fork of [yetone/magpie](https://github.com/yetone/magpie), versioned from 0.2.0. It differs from upstream in what it leaves out:

- **No telemetry.** No usage counts, install id, plugin likes or partner counts are sent.
- **No sponsors, partners or referral links.**
- **22 agents:** Claude Code, Claude Desktop, Claude Science, Codex, Gemini CLI, Antigravity CLI, OpenCode, Pi, Cursor CLI, Cursor Private Inference, Zed, VS Code Chat, VS Code Insiders, VSCodium Chat, Copilot CLI, Devin, Kimi Code, Muse Code, Grok Build, Grok App, Raycast, T3 Code.
- **No mainland-China providers or China-only features.** The overseas services of Chinese vendors (Z.ai, Kimi, MiniMax, StepFun, Qwen intl, Alibaba Cloud, Qwen Cloud, SiliconFlow, Tencent Cloud intl, Xiaomi MiMo's Singapore and Europe plans) stay. China mirrors and daily check-ins are gone.
- **English only.**
- **No website, Docker images or Windows builds.**

## Build and install (macOS)

Needs Go and Node.

```sh
make app VERSION=0.2.0
codesign --force --deep --sign - --identifier com.yetone.magpie magpie.app
ditto magpie.app /Applications/magpie.app
open /Applications/magpie.app
```

magpie runs in the menu bar. For the `magpie` command, use Settings → Command line, or link `/Applications/magpie.app/Contents/MacOS/magpie` into your PATH.

The app updates itself from upstream's release feed only to a newer version, and 0.2.0 is newer than every upstream 0.1.x release, so it never replaces this build. Turn automatic updates off in Settings → About to stop it checking at all.

## Using it

```sh
magpie provider add openrouter sk-or-…           # a preset needs only the key
magpie claude openrouter/anthropic/claude-sonnet-5-5
magpie codex moonshot/kimi-k3                    # Codex on Kimi
magpie group add "Opus anywhere" models=claude/claude-opus-5-5,copilot/claude-opus-5.5 routing=smart
magpie claude group/opus-anywhere                # fails over between subscriptions
magpie quota                                     # what's left on every plan
magpie tui
```

The full reference is [docs/reference.md](docs/reference.md); it is upstream's, and may still name things this fork removed.

## License

[MIT](LICENSE). magpie is not affiliated with [Apache Magpie](https://magpie.apache.org) or the [Apache Software Foundation](https://apache.org).
