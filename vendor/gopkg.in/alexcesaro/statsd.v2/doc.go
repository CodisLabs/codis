/*
Package statsd is a simple and efficient StatsD client.


Options

Use options to configure the Client: target host/port, sampling rate, tags, etc.

Whenever you want to use different options (e.g. other tags, different sampling
rate), you should use the Clone() method of the Client.

Because when cloning a Client, the same connection is reused so this is way
cheaper and more efficient than creating another Client using New().


Internals

Client's methods buffer metrics. The buffer is flushed when either:
 - the background goroutine flushes the buffer (every 100ms by default)
 - the buffer is full (1440 bytes by default so that IP packets are not
   fragmented)

The background goroutine can be disabled using the FlushPeriod(0) option.

Buffering can be disabled using the MaxPacketSize(0) option.

StatsD homepage: https://github.com/etsy/statsd
*/
package statsd
