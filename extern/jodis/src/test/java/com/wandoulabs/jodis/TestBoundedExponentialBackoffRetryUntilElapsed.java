/**
 * @(#)TestBoundedExponentialBackoffRetryUntilElapsed.java, 2014-12-2. 
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

import static org.junit.Assert.*;

import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;

import org.apache.curator.RetrySleeper;
import org.junit.Test;

/**
 * @author Apache9
 */
public class TestBoundedExponentialBackoffRetryUntilElapsed {

    private static final class FakeRetrySleeper implements RetrySleeper {

        public long sleepTimeMs;

        @Override
        public void sleepFor(long time, TimeUnit unit) {
            this.sleepTimeMs = unit.toMillis(time);
        }

    }

    @Test
    public void test() {
        FakeRetrySleeper sleeper = new FakeRetrySleeper();
        BoundedExponentialBackoffRetryUntilElapsed r = new BoundedExponentialBackoffRetryUntilElapsed(
                10, 2000, 60000);
        for (int i = 0; i < 100; i++) {
            assertTrue(r.allowRetry(i, ThreadLocalRandom.current().nextInt(60000), sleeper));
            System.out.println(sleeper.sleepTimeMs);
            assertTrue(sleeper.sleepTimeMs <= 2000);
        }
        assertTrue(r.allowRetry(1000, 59900, sleeper));
        System.out.println(sleeper.sleepTimeMs);
        assertTrue(sleeper.sleepTimeMs <= 100);
        sleeper.sleepTimeMs = -1L;
        assertFalse(r.allowRetry(1, 60000, sleeper));
        assertEquals(-1L, sleeper.sleepTimeMs);
    }
}
