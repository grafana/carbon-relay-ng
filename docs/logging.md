# log level description

* debug: state changes that we only need to know when debugging [1]
* info: for tracing data as it flows around ONLY, including discards (note that some tracing data can be in warnings too!) [1]
* notice: events worth noticing, i.e. harmless but they don't happen often or are special events like closing conn or manually triggered things [1]
* warning: something's wrong but we can survive, like write errors
* error:
* critical: not used, errors are critical

# fine-grained logging

logging related to instances of objects:
`{conn,dest,..} <spec>` where spec is typically the addr or listening port

# notes
[1] these metrics are potentially high volume and resource intensive
