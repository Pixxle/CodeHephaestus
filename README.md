<p align="center">
  <img src="assets/banner.png" alt="CodeHephaestus" width="700">
</p>

# CodeHephaestus

Autonomous task runner that picks up Jira issues, implements them with AI, and creates GitHub PRs — powered by [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview).

Continuously polls your task tracker, prioritizes work (review feedback → CI failures → new issues), manages git branches, and transitions Jira statuses through the full lifecycle.

## Install

```bash
pip install -e .
cp .env.example .env  # fill in your Jira credentials
```

## Usage

```
codehephaestus              # Run continuously (infinite loop)
codehephaestus --once       # Single iteration
codehephaestus --dry-run    # Show what would be done
codehephaestus -v           # Verbose (debug) logging
```

## License

[MIT](LICENSE)
