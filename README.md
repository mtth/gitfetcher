# Git fetcher [![codecov](https://codecov.io/gh/mtth/gitfetcher/graph/badge.svg?token=N1B8C8UMX0)](https://codecov.io/gh/mtth/gitfetcher)

> [!NOTE]
> WIP: this library is usable but its API may change in breaking ways.

A lightweight tool to create local copies of remote repositories.

Highlights:

* Simple file-based configuration
* `gitweb`-compatible local repositories
* Automation-friendly, including secret handling


## Quickstart

Sample configuration in [`txtpb` format][txtpb]:

```txtpb
# .gitfetcher
github {
  # Sync any public repository by name.
  sources { name: "golang/go" }
  sources { name: "mtth/gitfetcher" }

  # Sync all repositories available to a given authentication token.
  sources {
    auth {
      token: "$GITHUB_TOKEN" # Read from the environment at runtime
      include_forks: false # This is the default, and can be overridden
    }
  }

  # More sources...
}
```

Then run `gitfetcher sync .` in the folder containing the above configuration.


[txtpb]: https://protobuf.dev/reference/protobuf/textformat-spec/
