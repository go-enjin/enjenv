# Back Enjin Make

`enjenv` is a command line utility for managing local system environments.

Note that this documentation is likely incorrect at this time.

## Systems

A system is any software development environment such as `Golang` or `Nodejs`.

## Environment Variables

To configure the lowest parts of `enjenv` for your desired setup, the following
environment variables are available:

- `ENJENV_PATH`: explicit path to use for local environments
- `ENJENV_DIR_NAME`: base directory name, `.be` by default
- `ENJENV_GOLANG_TAG`: tag for other Golang variables
  - `ENJENV_{GOLANG_TAG}_NAME`: base directory name for the Golang system, `golang` by default
  - `ENJENV_{GOLANG_TAG}_BIN_NAME`: name of the binary for Golang to produce when using `enjenv golang build`
  - `ENJENV_{GOLANG_TAG}_SUMMARY`: sets the `github.com/go-enjin/be/pkg/globals.Summary` build flag
  - `ENJENV_{GOLANG_TAG}_VERSION`: sets the `github.com/go-enjin/be/pkg/globals.Version` build flag
  - `ENJENV_{GOLANG_TAG}_ENV_PREFIX`: sets the `github.com/go-enjin/be/pkg/globals.EnvPrefix` build flag
  - `ENJENV_{GOLANG_TAG}_TMP_DIR_NAME`: name of the tmp dir for the Golang system, `tmp` by default
  - `ENJENV_{GOLANG_TAG}_CACHE_DIR_NAME`: name of the cache dir for the Golang system `cache` by default
  - `ENJENV_{GOLANG_TAG}_MOD_CACHE_DIR_NAME`: name of the modcache dir for the Golang system `modcache` by default
  - `ENJENV_DEFAULT_{GOLANG_TAG}_VERSION`: default version of Golang to install, `1.17.7` by default
  - `BE_LOCAL_PATH`: path to a local `github.com/go-enjin/be` checkout to use for `enjenv golang mod <local|unlocal>`
- `ENJENV_NODEJS_TAG`: tag for other Nodejs variables
  - `ENJENV_{NODEJS_TAG}_NAME`: base directory name for the Nodejs system, `nodejs` by default
  - `ENJENV_{NODEJS_TAG}_CACHE_DIR_NAME`: name of the cache dir for the Nodejs system, `nodecache` by default
  - `ENJENV_DEFAULT_{NODEJS_TAG}_VERSION`: default version of Nodejs to install, `16.14.0` by default

## Install

To install `enjenv`, use the normal go methods.

## Help

When run for the first time, a limited set of commands are available. There are
two primary commands, `golang` and `nodejs`. Both of which have only one
sub-command of note: `init`.

On the top-level, there's one convenience command available which is just a
wrapper around running `init` for all systems available.

### First Run Help

```shell
$ enjenv --help
NAME:
   enjenv - local golang installation and environment utility

USAGE:
   enjenv [global options]

VERSION:
   v0.0.2

COMMANDS:
   golang   work with a local golang environment
   nodejs   work with a local nodejs environment
   init     create local environments

GLOBAL OPTIONS:
   --help, -h     show help (default: false)
   --version, -v  print the version (default: false)
```

#### Golang

```shell
$ enjenv golang --help
NAME:
   enjenv golang - work with a local golang environment

USAGE:
   enjenv golang [global options] command [command options] [arguments...]

COMMANDS:
   init     create a local golang environment

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

#### Nodejs

```shell
$ enjenv nodejs --help
NAME:
   enjenv nodejs - work with a local nodejs environment

USAGE:
   enjenv nodejs [global options] command [command options] [arguments...]

COMMANDS:
   init     create a local nodejs environment

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

### Init Help

The `init` commands essentially do the following steps for their respective
systems (stopping at any point an error is raised):

1. download, or use, a given archive
2. retrieve a list of sha256sums from hard-coded URLs
3. validate the sha256sum of the archive to use (from step 1)
4. extract the archive
5. create additional directories to support system-specific settings

Once `init` has completed, new `enjenv` features become available.

### Installed Features Help

### Golang

```shell
$ enjenv golang --help
NAME:
   enjenv golang - work with a local golang environment

USAGE:
   enjenv golang [global options] command [command options] [arguments...]

COMMANDS:
   init      create a local golang environment
   version   reports the installed golang version
   clean     delete the local golang environment
   export    output shell export statements
   unexport  output shell unset statements (inverse of export)
   mod       helper features for working with go.mod
   build     local go build convenience wrapper

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

### Nodejs

```shell
$ enjenv nodejs --help
NAME:
   enjenv nodejs - work with a local nodejs environment

USAGE:
   enjenv nodejs [global options] command [command options] [arguments...]

COMMANDS:
   init      create a local nodejs environment
   version   reports the installed nodejs version
   clean     delete the local nodejs environment
   export    output shell export statements
   unexport  output shell unset statements (inverse of export)

GLOBAL OPTIONS:
   --help, -h  show --help (default: false)
```

### Export / UnExport Help

Each system provides a set of environment variables which force the use of the
specific installation setup with two helper features to support a developer's
use of the system: `export` and `unexport`. The former prints to STDOUT lines of
`export KEY=value` statements and the latter prints `unset KEY;` statements.

Examine the output of the commands first to get an idea of the content present
and then use them by: `eval $(enjenv export)`,
`enjenv export > .env ; source .env` or whatever method is best.

One feature to note is that the `PATH` variable is included in both the `export`
and `unexport` output and does what one would expect. After sourcing these
variables, a developer can now use the normal system tools without needing to
prefix everything with `enjenv` calls.

### Additional Golang Features

The Golang system include two extra features: `mod` and `build`. Both of these
are utilities specific to building other Back Enjin services.

#### enjenv golang mod help

```shell
$ enjenv golang mod --help
NAME:
   enjenv golang mod - helper features for working with go.mod

USAGE:
   enjenv golang mod [global options] command [command options] [arguments...]

COMMANDS:
   local    go mod edit -replace wrapper
   unlocal  go mod edit -dropreplace wrapper
   tidy     go mod tidy wrapper
   init     go mod init wrapper

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

##### Local / Un-local Help

These two commands are are simple semantic wrappers around the normal
`go mod edit` `-replace` and `-dropreplace` actions.

```shell
$ enjenv golang mod local --help
NAME:
   enjenv golang mod local - go mod edit -replace wrapper

USAGE:
   
  # set an arbitrary package name to be replaced with given path
  enjenv golang mod local [any/package/name path/to/checkout]

  # set github.com/go-enjin/be to be replaced with given path
  enjenv golang mod local [path/to/go-enjin/be]

  # set github.com/go-enjin/be to be replaced with the $BE_LOCAL_PATH environment variable
  enjenv golang mod local

OPTIONS:
   --help, -h  show help (default: false)
```
