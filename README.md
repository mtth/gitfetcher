# Git fetcher [![codecov](https://codecov.io/gh/mtth/gitfetcher/graph/badge.svg?token=N1B8C8UMX0)](https://codecov.io/gh/mtth/gitfetcher)

> [!NOTE]
> WIP: this tool is usable but its API may change in breaking ways.

A lightweight CLI to create local copies of remote repositories.

Highlights:

* Simple file-based configuration
* `gitweb`-compatible local repositories
* Automation-friendly, including secret handling


## Quickstart


```sh
go install github.com/mtth/gitfetcher
```

Sample `.gitfetcher` configuration ([`txtpb` format][txtpb]):

```pbtxt
  # Sync public repositories from their. URL
sources { from_url { url: "https://github.com/golang/go" }}
sources { from_url { url: "https://github.com/nodejs/node" }}

# Sync repositories available to a given GitHub authentication token. This is
# useful for example to sync all your personal repos.
sources {
  from_github_token {
    # A token with read access to repositories is required. It can either be
    # specified inline or via an environment variable (prefixing it with `$`).
    token: "$GITHUB_TOKEN"

    # Forks are excluded by default and can be included using via this option.
    # include_forks: true

    # It's also possible to filter by repository name by specifying one or
    # more filters, optionally including wildcards. A repository will be
    # synced if it matches at least one.
    # filters: "user/*"
    # filters: "user/prefix*"
  }
}

# More sources...

# Path to root folder where local repositories will be stored, relative to the
# configuration file. Defaults to the configuration's enclosing directory.
# root: "/path/to/.gitfetcher"
```

Then run `gitfetcher sync` in the folder containing the above configuration.
See `gitfetcher --help` for the full list of available commands and options.


[txtpb]: https://protobuf.dev/reference/protobuf/textformat-spec/
