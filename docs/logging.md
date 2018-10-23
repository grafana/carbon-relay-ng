# log level description

* trace: for tracing messages from start to finish, including unroutable/discards [1]
* debug: state changes that we only need to know when debugging, client conns opening and closing [1]
* info:  harmless, but interesting not-so-common events. e.g. outbound connection changes, manually triggered flushes, etc. (this used to be `notice`)
* warn:  minor issues (network timeouts etc)
* error: recoverable errors
* fatal: errors and problems that result in shutdown
* panic: not used

# fine-grained logging

logging related to instances of objects:
`{conn,dest,..} <spec>` where spec is typically the addr or listening port

# notes
[1] these metrics are potentially high volume and resource intensive
