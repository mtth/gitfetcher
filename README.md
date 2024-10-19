# Git fetcher

> [!NOTE]
> WIP

A simple binary to create local copies of remote repositories.

Highlights:

* Single-file configuration
* `gitweb`-compatible clones
* Automation-friendly


## Quickstart

Sample configuration:

```txtpb
# .gitfetcher
github {
  # Sync any public repository by name.
  sources { name: "golang/go" }
  sources { name: "mtth/gitfetcher" }

  # Sync all repositories available to a given authentication token.
  sources {
    auth {
      token: "$GITHUB_TOKEN"
      include_forks: false # This is the default, and can be overridden.
    }
  }

  # More sources...
}
```

Then run `gitfetcher sync .` in the folder containing the above configuration.
