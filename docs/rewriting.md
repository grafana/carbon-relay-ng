
Series names can be rewritten as they pass through the system by Rewriter rules, which are processed in series.

Basic rules use simple old/new text replacement, and support a Max parameter to specify the maximum number of matched items to be replaced.

Rewriter rules also support regexp syntax, which is enabled by wrapping the "old" parameter with forward slashes and setting "max" to -1.

Regexp rules support [golang's standard regular expression syntax](https://golang.org/pkg/regexp/syntax/), and the "new" value can include [submatch identifiers](https://golang.org/pkg/regexp/#Regexp.Expand) in the format `${1}`.

Examples (using init commands. you can also specify them in the config directly. see the config docs)

```
# basic rewriter rule to replace first occurrence of "foo" with "bar"
addRewriter foo bar 1

# regexp rewriter rule to add a prefix of "prefix." to all series
addRewriter /^/ prefix. -1

# regexp rewriter rule to replace "server.X" with "servers.X.collectd"
addRewriter /server\.([^.]+)/ servers.${1}.collectd -1
```

