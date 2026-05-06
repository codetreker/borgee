# @codetreker/borgee-remote-agent

CLI tool for connecting remote file systems to [Borgee](https://borgee.codetrek.cn) channels.

## Install

```bash
npx @codetreker/borgee-remote-agent --server wss://borgee.codetrek.cn --token <connection_token> --dirs /path/to/dir
```

## Usage

```bash
borgee-remote-agent --server <wss://...> --token <token> --dirs <path1> [--dirs <path2>]
```

The agent connects via WebSocket and exposes local directories for browsing and file access within Borgee channels.
