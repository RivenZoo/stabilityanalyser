# Intro

Do statistics to module dependencies. Count fan-in/fan-out and calculate volatile metrics. Output result json or digraph.

# Install

`go get github.com/RivenZoo/stabilityanalyser`

# Usage

```
▶ stabilityanalyser analyse -h
Receive module dependency from stdin, do statistics about module dependency fan-in and fan-out.
Module dependency described as ["module_name_A" -> "module_name_B";], one item per line.

Usage:
  stabilityanalyser analyse [flags]

Flags:
      --digraph        output digraph or not
  -h, --help           help for analyse
      --limit int      limit output, order should be set. default no limit
      --order string   order by [fan-in | fan-out | volatile]

Global Flags:
      --config string   config file
```

You can use another tool [godepgraph](https://github.com/kisielk/godepgraph) to parse module dependency as input of this
tool.

```
godepgraph github.com/RivenZoo/stabilityanalyser | \
    grep '\->' | \
    stabilityanalyser analyse --order volatile --limit 30 --digraph | \
    dot -Tpng -o volatile-dep.png
```