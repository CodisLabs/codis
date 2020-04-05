start_server {tags {"slowlog"} overrides {slowlog-log-slower-than 1000000}} {
    test {SLOWLOG - check that it starts with an empty log} {
        r slowlog len
    } {0}

    test {SLOWLOG - only logs commands taking more time than specified} {
        r config set slowlog-log-slower-than 100000
        r ping
        assert_equal [r slowlog len] 0
        r debug sleep 0.2
        assert_equal [r slowlog len] 1
    }

    test {SLOWLOG - max entries is correctly handled} {
        r config set slowlog-log-slower-than 0
        r config set slowlog-max-len 10
        for {set i 0} {$i < 100} {incr i} {
            r ping
        }
        r slowlog len
    } {10}

    test {SLOWLOG - GET optional argument to limit output len works} {
        llength [r slowlog get 5]
    } {5}

    test {SLOWLOG - RESET subcommand works} {
        r config set slowlog-log-slower-than 100000
        r slowlog reset
        r slowlog len
    } {0}

    test {SLOWLOG - logged entry sanity check} {
        r client setname foobar
        r debug sleep 0.2
        set e [lindex [r slowlog get] 0]
        assert_equal [llength $e] 6
        assert_equal [lindex $e 0] 105
        assert_equal [expr {[lindex $e 2] > 100000}] 1
        assert_equal [lindex $e 3] {debug sleep 0.2}
        assert_equal {foobar} [lindex $e 5]
    }

    test {SLOWLOG - commands with too many arguments are trimmed} {
        r config set slowlog-log-slower-than 0
        r slowlog reset
        r sadd set 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33
        set e [lindex [r slowlog get] 0]
        lindex $e 3
    } {sadd set 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 {... (2 more arguments)}}

    test {SLOWLOG - too long arguments are trimmed} {
        r config set slowlog-log-slower-than 0
        r slowlog reset
        set arg [string repeat A 129]
        r sadd set foo $arg
        set e [lindex [r slowlog get] 0]
        lindex $e 3
    } {sadd set foo {AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA... (1 more bytes)}}

    test {SLOWLOG - EXEC is not logged, just executed commands} {
        r config set slowlog-log-slower-than 100000
        r slowlog reset
        assert_equal [r slowlog len] 0
        r multi
        r debug sleep 0.2
        r exec
        assert_equal [r slowlog len] 1
        set e [lindex [r slowlog get] 0]
        assert_equal [lindex $e 3] {debug sleep 0.2}
    }

    test {SLOWLOG - can clean older entires} {
        r client setname lastentry_client
        r config set slowlog-max-len 1
        r debug sleep 0.2
        assert {[llength [r slowlog get]] == 1}
        set e [lindex [r slowlog get] 0]
        assert_equal {lastentry_client} [lindex $e 5]
    }

    test {SLOWLOG - can be disabled} {
        r config set slowlog-max-len 1
        r config set slowlog-log-slower-than 1
        r slowlog reset
        r debug sleep 0.2
        assert_equal [r slowlog len] 1
        r config set slowlog-log-slower-than -1
        r slowlog reset
        r debug sleep 0.2
        assert_equal [r slowlog len] 0
    }
}
