
== these messages are potentially high volume and resource intensive ==
debug: state changes that we only need to know when debugging
info: for tracing data as it flows around ONLY, including discards (note that some tracing data can be in warnings too!)
notice: events worth noticing, i.e. harmless but they don't happen often or are special events like closing conn or manually triggered things

== everything from here on is low volume only ==
== we don't want to spend significant time in the logging library and/or being very spammy/resource intensive ==

warning: something's wrong but we can survive, like write errors
error:
critical: not used, errors are critical

logging related to instances of objects:
{conn,dest,..} <spec> where spec is typically the addr or listening port
