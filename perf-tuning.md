You should do some tuning to avoid the "dropped due to slow conn" and "dropped due to slow spool" warnings.


tuning outbound tcp conn to destination
--------------------------------

(see the constants in conn.go and the flush interval option per dest)

every destination has a bufio writer, with a configurable flush (but it will also autoflush during write it goes full, better to avoid this by making buffer big enough),
since flushes and writes (see above, to be avoided) can take a while, there's also a buffered channel sitting in front of it to make sure the conn can always take writes
in a non-blocking fashion.
when the chan runs full, it means we can't write fast enough we drop those metrics, so keep an eye on those "dropped due to slow conn" warning messages on stdout and the internal metric for it (every dest has a numBuffered metric)

Below you'll find some notes on how I did my testing.  The approach (and results) might be interesting to you.
But the bottom line is, the auto-flushes and writes in worst case (when i saw 10k metric/s peaks) took much longer than i wanted them to
(some writes/flushes took way too long for the amount of data, compared to the network speed)
peak traffic 10k metrics/s =~ 8Mbps should be able to be flushed in realtime..

I tried
* fiddling with flush interval (every 100ms or every 1s) without results
* setting bufio_buffer_size to 2000000 to prevent writer flushing by itself, for at least 2 consecutive seconds of peak, assume 100 bytes for each packet  -> this by itself did not work.  flush went up to 4s!


As i couldn't tune this properly, I eventually just decided to increase the buffer to deal with the peaks, even though in theory this shouldn't be needed.

*Hopefully we can revisit this and tune performance better for everyone to get high troughput without drops.*

my personal notes
-----------------

here are some notes that i keep, in case the approach is useful to replicate.

amount of packets/s :
looking per second (ngrep | linecounter): rough visual avg 3000 (180k/minute), peak of 10k per s (extrapolated 600k/minute)
looking per 0.1s   (ngrep | linecounter): rough visual avg  300 (180k/minute), peak upto 2500k/0.1s (extrapolated 1.5M/minute)
looking per 0.01s  (ngrep | linecounter):                           peak of 700/0.01s (extrapolated 4.2M/minute)
on micros scale (timestamps in debug log) very short burst peaks: 1 every few micros (extrapolated: ~500k metrics/s or 30M/minute)
also we saw 2031010 metrics in 455s i.e. 4463 writes/s which confirms the rough avg above (270k/minute)
HandleData timings:
write                 avg 10micros, typically 8-9, but occasionally upto 35micros -> should support 100k metrics/s
autoflush (every 1s): avg 50micros, typical between 20 and 200, but sometimes upto 4ms -> should support about 20k metrics/s
also for every 2031010 writes, i saw 479 auto-flush (and no other handleData's), so that's 4240 writes per auto-flush (with flush per 1000ms)
so for every 4240 *9micros = 38ms of writing there is around 200micros (upto 4ms) of autoflush, so writing is the slowest, but flush can block longer.
in 4ms (max flush) we can get up to 280 metrics (700/0.01s to 0.004s)
in grafana real world metrics at peak:
9k/s for 2 consecutive seconds, no errors, but causing a buffer overload (1k) resulting in drops
flush peak at 200 micros, write peak at 165ms for 2 consecutive seconds -> presumably because it flushes by itself, but why so long :(
flush 99   at 200 micros, write 99   at 100 micros for 1 second
flush mean at 200 micros, write mean at 175 micros for 1 second



tuning of spooling
------------------

similar as before, look at the metrics to see how long it takes to handle writes to the spool file,
and update the values in spool.go accordingly.
