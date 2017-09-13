# routes

## carbon route

setting        | mandatory | values                                        | default | description 
---------------|-----------|-----------------------------------------------|---------|------------
key            |     Y     | string                                        | N/A     |
type           |     Y     | sendAllMatch/sendFirstMatch/consistentHashing | N/A     | send to all destinations vs first matching destination vs distribute via consistent hashing
prefix         |     N     | string                                        | ""      |
sub            |     N     | string                                        | ""      |
regex          |     N     | string                                        | ""      |

## carbon destination

setting              | mandatory | values        | default | description 
---------------------|-----------|---------------|---------|------------
addr                 |     Y     |  string       | N/A     |
prefix               |     N     |  string       | ""      |
sub                  |     N     |  string       | ""      |
regex                |     N     |  string       | ""      |
flush                |     N     |  int (ms)     | 1000    | flush interval
reconn               |     N     |  int (ms)     | 10k     | reconnection interval
pickle               |     N     |  true/false   | false   | pickle output format instead of the default text protocol
spool                |     N     |  true/false   | false   | disk spooling
connbuf              |     N     |  int          | 30k     | connection buffer (how many metrics can be queued, not written into network conn)
iobuf                |     N     |  int (bytes)  | 2M      | buffered io connection buffer
spoolbuf             |     N     |  int          | 10k     | num of metrics to buffer across disk-write stalls. practically, tune this to number of metrics in a second
spoolmaxbytesperfile |     N     |  int          | 200MiB  | max filesize for spool files
spoolsyncevery       |     N     |  int          | 10k     | sync spool to disk every this many metrics
spoolsyncperiod      |     N     |  int  (ms)    | 1000    | sync spool to disk every this many milliseconds
spoolsleep           |     N     |  int (micros) | 500     | sleep this many microseconds(!) in between ingests from bulkdata/redo buffers into spool
unspoolsleep         |     N     |  int (micros) | 10      | sleep this many microseconds(!) in between reads from the spool, when replaying spooled data

## grafanaNet route

setting        | mandatory | values      | default | description 
---------------|-----------|-------------|---------|------------
key            |     Y     |  string     | N/A     |
addr           |     Y     |  string     | N/A     |
apiKey         |     Y     |  string     | N/A     |
schemasFile    |     Y     |  string     | N/A     |
prefix         |     N     |  string     | ""      |
sub            |     N     |  string     | ""      |
regex          |     N     |  string     | ""      |
sslverify      |     N     |  true/false | true    |
spool          |     N     |  true/false | false   | ** disk spooling. not implemented yet **
blocking       |     N     |  true/false | false   | if false, full buffer drops data. if true, full buffer puts backpressure on the table, possibly affecting ingestion and other routes
concurrency    |     N     |  int        | 10      | number of concurrent connections to ingestion endpoint
bufSize        |     N     |  int        | 10M     | buffer size. assume +- 100B per message, so 10M is about 1GB of RAM
flushMaxNum    |     N     |  int        | 10k     | max number of metrics to buffer before triggering flush
flushMaxWait   |     N     |  int (ms)   | 500     | max time to buffer before triggering flush
timeout        |     N     |  int (ms)   | 5000    | abort and retry requests to api gateway if takes longer than this.
orgId          |     N     |  int        | 1       |

## kafkaMdm route

setting        | mandatory | values      | default | description 
---------------|-----------|-------------|---------|------------
key            |     Y     |  string     | N/A     |
brokers        |     Y     |  []string   | N/A     |
topic          |     Y     |  string     | N/A     |
codec          |     Y     |  string     | N/A     |
partitionBy    |     Y     |  string     | N/A     |
schemasFile    |     Y     |  string     | N/A     |
prefix         |     N     |  string     | ""      |
sub            |     N     |  string     | ""      |
regex          |     N     |  string     | ""      |
blocking       |     N     |  true/false | false   | if false, full buffer drops data. if true, full buffer puts backpressure on the table, possibly affecting ingestion and other routes
bufSize        |     N     |  int        | 10M     | buffer size. assume +- 100B per message, so 10M is about 1GB of RAM
flushMaxNum    |     N     |  int        | 10k     | max number of metrics to buffer before triggering flush
flushMaxWait   |     N     |  int (ms)   | 500     | max time to buffer before triggering flush
timeout        |     N     |  int (ms)   | 2000    |
orgId          |     N     |  int        | 1       |

