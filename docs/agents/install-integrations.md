# Install the Codex plugin or Claude Code skill

Tapioca ships one plugin package that works with both Codex and Claude Code:
`plugins/tapioca-local-ai`. It contains the shared `use-tapioca` Agent Skill,
reference notes, and a structured helper script.

The plugin teaches an agent how to inspect the installed catalog, choose a
compatible local model, use Tapioca's APIs, generate media, and clean up
processes it started. It does not install Tapioca or bypass the agent's normal
tool permissions.

## Before you begin

1. Install Tapioca from the
   [latest release](https://github.com/kyndlo/tapioca/releases/latest).
2. Confirm it is available:

   ```bash
   tapioca version
   tapioca catalog update
   tapioca catalog
   ```

3. Clone this repository if you want to test the integration from source:

   ```bash
   git clone https://github.com/kyndlo/tapioca.git
   cd tapioca
   ```

## Codex

Add Tapioca's repository marketplace, then install the plugin:

```bash
codex plugin marketplace add https://github.com/kyndlo/tapioca
codex plugin add tapioca-local-ai@personal
```

Restart Codex after installation. Ask Codex to use Tapioca, or explicitly
mention the `use-tapioca` skill. Codex loads the skill only when the task needs
local model or media operations.

To inspect or remove the installation:

```bash
codex plugin list
codex plugin remove tapioca-local-ai@personal
```

## Claude Code

Test the plugin directly from a repository checkout:

```bash
claude --plugin-dir ./plugins/tapioca-local-ai
```

Inside Claude Code, invoke the skill explicitly:

```text
/tapioca-local-ai:use-tapioca
```

You can also ask naturally, for example: “Use Tapioca to select and run a
local tool-capable model for this repository.”

For a durable team installation, add this plugin directory to your
organization's Claude Code plugin marketplace. The package includes
`.claude-plugin/plugin.json` and follows the standard
`skills/<name>/SKILL.md` layout.

## Structured helper

Agents can call the helper when they need machine-readable status or managed
server lifecycle:

```bash
python3 plugins/tapioca-local-ai/skills/use-tapioca/scripts/tapioca_agent.py detect
python3 plugins/tapioca-local-ai/skills/use-tapioca/scripts/tapioca_agent.py start MODEL
python3 plugins/tapioca-local-ai/skills/use-tapioca/scripts/tapioca_agent.py health
python3 plugins/tapioca-local-ai/skills/use-tapioca/scripts/tapioca_agent.py stop
```

Every command prints JSON. The helper stores only its own server state under
`~/.tapioca/agent-helper` unless `TAPIOCA_AGENT_STATE` points elsewhere.

## Verify the package

Maintainers can validate both manifests and the Agent Skill with:

```bash
python3 /path/to/plugin-creator/scripts/validate_plugin.py plugins/tapioca-local-ai
python3 /path/to/skill-creator/scripts/quick_validate.py plugins/tapioca-local-ai/skills/use-tapioca
```

The public agent contract is also available at
[tapioca.rootfruit.cc/llm](https://tapioca.rootfruit.cc/llm),
[llms.txt](https://tapioca.rootfruit.cc/llms.txt), and
[llms-full.txt](https://tapioca.rootfruit.cc/llms-full.txt).
