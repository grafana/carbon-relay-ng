## Rewriting

Series names can be rewritten as they pass through the system by Rewriter rules, which are processed in series.
This can be done via regex (expensive), or with a string check (cheap)

## With string check

Basic rules use simple old/new text replacement, and support a Max parameter to specify the maximum number of matched items to be replaced.

## With regexular expression

This is activated by wrapping the "old" parameter with forward slashes and setting "max" to -1.
The "new" value can include [submatch identifiers](https://golang.org/pkg/regexp/#Regexp.Expand) in the format `${1}`.

Note that for performance reasons, these regular expressions don't support lookaround (lookahead, lookforward)
For more information see:

* [this ticket](https://github.com/StefanSchroeder/Golang-Regex-Tutorial/issues/11) 
* [syntax documentation](https://github.com/google/re2/wiki/Syntax)
* [golang's regular expression package documentation](https://golang.org/pkg/regexp/syntax/)

## Examples

### Using the new config style

This is the recommended approach.
The new config style also supports an extra field: `not`: skips the rewriting if the metric matches the pattern.
The pattern can either be a substring, or a regex enclosed in forward slashes.

```
# basic rewriter rule to replace first occurrence of "foo" with "bar"
[[rewriter]]
old = 'foo'
new = 'bar'
not = ''
max = -1

# regexp rewriter rule to add a prefix of "prefix." to all series
[[rewriter]]
old = '/^/'
new = 'prefix.'
not = ''
max = -1

# regexp rewriter rule to replace "server.X" with "servers.X.collectd"
[[rewriter]]
old = '/server\.([^.]+)/'
new = 'servers.${1}.collectd'
not = ''
max = -1

# same, except don't do it if the metric already contains the word collectd
[[rewriter]]
old = '/server\.([^.]+)/'
new = 'servers.${1}.collectd'
not = 'collectd'
max = -1
```


### Using init commands

(deprecated)

```
# basic rewriter rule to replace first occurrence of "foo" with "bar"
addRewriter foo bar 1

# regexp rewriter rule to add a prefix of "prefix." to all series
addRewriter /^/ prefix. -1

# regexp rewriter rule to replace "server.X" with "servers.X.collectd"
addRewriter /server\.([^.]+)/ servers.${1}.collectd -1
```

