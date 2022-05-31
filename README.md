## portbump

Bump port revisions.

#### Installation

    go install github.com/dmgk/portbump

#### Usage

```
Usage: portbump [-R path][-qhV] [category/port ...]

Bump port revisions.

Options:
  -R path        ports tree root (default: /usr/ports)
  -q             be quiet
  -h             print help and exit
  -V             print version and exit

Arguments:
  category/port  port origin(s) to bump PORTREVISION of

  Alternatively, pipe a space separated category/port list
  (e.g. from "portgrep -1" to the portbump standard input.
```

#### Examples

Bump Go ports PORTREVISION:

```sh
portbump lang/go{,117,-devel}
```

Bump PORTREVISION of all USES=go ports:

```sh
$ portgrep -u go -1 | portbump
```

Bump PORTREVISION of all ports depending on devel/libcjson:

```sh
$ portgrep -dl libcjson.so -1 | portbump
```
