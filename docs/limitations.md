Limitations
-----------

* regex rewriter rules do not support limiting number of replacements, max must be set to -1
* the web UI is not always reliable to make changes.  the config file and tcp interface are safer and more complete anyway.
* internal metrics *must* be routed somewhere (e.g. into the relay itself) otherwise it'll [leak memory](https://github.com/grafana/carbon-relay-ng/issues/50).
  this is a silly bug but I haven't had time yet to fix it.

