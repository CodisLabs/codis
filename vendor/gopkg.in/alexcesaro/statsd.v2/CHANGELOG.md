# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [2.0.0] - 2016-03-20

- `New` signature changed. The default address used is now ":8125". To use
  another address use the `Address` option:

  Before:
  ```
  statsd.New(":8125")
  statsd.New(":9000")
  ```

  After
  ```
  statsd.New()
  statsd.New(statsd.Address(":9000"))
  ```

- The `rate` parameter has been removed from the `Count` and `Timing` methods.
  Use the new `SampleRate` option instead.

- `Count`, `Gauge` and `Timing` now accept a `interface{}` instead of an int as
  the value parameter. So you can now use any type of integer or float in these
  functions.

- The `WithInfluxDBTags` and `WithDatadogTags` options were replaced by the
  `TagsFormat` and `Tags` options:

  Before:
  ```
  statsd.New(statsd.WithInfluxDBTags("tag", "value"))
  statsd.New(statsd.WithDatadogTags("tag", "value"))
  ```

  After
  ```
  statsd.New(statsd.TagsFormat(statsd.InfluxDB), statsd.Tags("tag", "value"))
  statsd.New(statsd.TagsFormat(statsd.Datadog), statsd.Tags("tag", "value"))
  ```

- All options whose named began by `With` had the `With` stripped:

  Before:
  ```
  statsd.New(statsd.WithMaxPacketSize(65000))
  ```

  After
  ```
  statsd.New(statsd.MaxPacketSize(65000))
  ```

- `ChangeGauge` has been removed as it is a bad practice: UDP packets can be
  lost so using relative changes can cause unreliable values in the long term.
  Use `Gauge` instead which sends an absolute value.

- The `Histogram` method has been added.

- The `Clone` method was added to the `Client`, it allows to create a new
  `Client` with different rate / prefix / tags parameters while still using the
  same connection.
