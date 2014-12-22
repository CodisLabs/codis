/**
 * @(#)BoundedExponentialBackoffRetryUntilElapsed.java, 2014-12-2. 
 * 
 * Copyright (c) 2014 Wandoujia Inc.
 * 
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
 * LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
 * OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
 * WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
package com.wandoulabs.jodis;

import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;

import org.apache.curator.RetryPolicy;
import org.apache.curator.RetrySleeper;

/**
 * Similar to {@link org.apache.curator.retry.BoundedExponentialBackoffRetry},
 * but limit the retry elapsed time, not retry number.
 * 
 * @author Apache9
 */
public class BoundedExponentialBackoffRetryUntilElapsed implements RetryPolicy {

    private final int baseSleepTimeMs;

    private final int maxSleepTimeMs;

    private final long maxElapsedTimeMs;

    /**
     * @param baseSleepTimeMs
     *            initial amount of time to wait between retries
     * @param maxSleepTimeMs
     *            max time in ms to sleep on each retry
     * @param maxElapsedTimeMs
     *            total time in ms to retry
     */
    public BoundedExponentialBackoffRetryUntilElapsed(int baseSleepTimeMs, int maxSleepTimeMs,
            long maxElapsedTimeMs) {
        this.baseSleepTimeMs = baseSleepTimeMs;
        this.maxSleepTimeMs = maxSleepTimeMs;
        if (maxElapsedTimeMs < 0) {
            this.maxElapsedTimeMs = Long.MAX_VALUE;
        } else {
            this.maxElapsedTimeMs = maxElapsedTimeMs;
        }
    }

    private long getSleepTimeMs(int retryCount, long elapsedTimeMs) {
        return Math.min(
                maxSleepTimeMs,
                (long) baseSleepTimeMs
                        * Math.max(
                                1,
                                ThreadLocalRandom.current().nextInt(
                                        1 << Math.min(30, retryCount + 1))));
    }

    @Override
    public boolean allowRetry(int retryCount, long elapsedTimeMs, RetrySleeper sleeper) {
        if (elapsedTimeMs >= maxElapsedTimeMs) {
            return false;
        }
        long sleepTimeMs = Math.min(maxElapsedTimeMs - elapsedTimeMs,
                getSleepTimeMs(retryCount, elapsedTimeMs));
        try {
            sleeper.sleepFor(sleepTimeMs, TimeUnit.MILLISECONDS);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return false;
        }
        return true;
    }
}
