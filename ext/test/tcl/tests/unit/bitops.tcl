# Compare Redis commadns against Tcl implementations of the same commands.
proc count_bits s {
    binary scan $s b* bits
    string length [regsub -all {0} $bits {}]
}

proc simulate_bit_op {op args} {
    set maxlen 0
    set j 0
    set count [llength $args]
    foreach a $args {
        binary scan $a b* bits
        set b($j) $bits
        if {[string length $bits] > $maxlen} {
            set maxlen [string length $bits]
        }
        incr j
    }
    for {set j 0} {$j < $count} {incr j} {
        if {[string length $b($j)] < $maxlen} {
            append b($j) [string repeat 0 [expr $maxlen-[string length $b($j)]]]
        }
    }
    set out {}
    for {set x 0} {$x < $maxlen} {incr x} {
        set bit [string range $b(0) $x $x]
        if {$op eq {not}} {set bit [expr {!$bit}]}
        for {set j 1} {$j < $count} {incr j} {
            set bit2 [string range $b($j) $x $x]
            switch $op {
                and {set bit [expr {$bit & $bit2}]}
                or  {set bit [expr {$bit | $bit2}]}
                xor {set bit [expr {$bit ^ $bit2}]}
            }
        }
        append out $bit
    }
    binary format b* $out
}

start_server {tags {"bitops"}} {

    r del no-key str foo
    
    test {BITCOUNT returns 0 against non existing key} {
        r bitcount no-key
    } 0

    catch {unset num}
    foreach vec [list "" "\xaa" "\x00\x00\xff" "foobar" "123"] {
        incr num
        test "BITCOUNT against test vector #$num" {
            r set str $vec
            assert {[r bitcount str] == [count_bits $vec]}
        }
    }

    test {BITCOUNT fuzzing without start/end} {
        for {set j 0} {$j < 100} {incr j} {
            set str [randstring 0 3000]
            r set str $str
            assert {[r bitcount str] == [count_bits $str]}
        }
    }

    test {BITCOUNT fuzzing with start/end} {
        for {set j 0} {$j < 100} {incr j} {
            set str [randstring 0 3000]
            r set str $str
            set l [string length $str]
            set start [randomInt $l]
            set end [randomInt $l]
            if {$start > $end} {
                lassign [list $end $start] start end
            }
            assert {[r bitcount str $start $end] == [count_bits [string range $str $start $end]]}
        }
    }

    test {BITCOUNT with start, end} {
        r set s "foobar"
        assert_equal [r bitcount s 0 -1] [count_bits "foobar"]
        assert_equal [r bitcount s 1 -2] [count_bits "ooba"]
        assert_equal [r bitcount s -2 1] [count_bits ""]
        assert_equal [r bitcount s 0 1000] [count_bits "foobar"]
    }

    test {BITCOUNT syntax error #1} {
        catch {r bitcount s 0} e
        set e
    } {ERR*syntax*}

    test {BITCOUNT regression test for github issue #582} {
        r del str
        r setbit foo 0 1
        if {[catch {r bitcount foo 0 4294967296} e]} {
            assert_match {*ERR*out of range*} $e
            set _ 1
        } else {
            set e
        }
    } {1}

    test {BITCOUNT misaligned prefix} {
        r del str
        r set str ab
        r bitcount str 1 -1
    } {3}

    test {BITCOUNT misaligned prefix + full words + remainder} {
        r del str
        r set str __PPxxxxxxxxxxxxxxxxRR__
        r bitcount str 2 -3
    } {74}

    test {BITPOS bit=0 with empty key returns 0} {
        r del str
        r bitpos str 0
    } {0}

    test {BITPOS bit=1 with empty key returns -1} {
        r del str
        r bitpos str 1
    } {-1}

    test {BITPOS bit=0 with string less than 1 word works} {
        r set str "\xff\xf0\x00"
        r bitpos str 0
    } {12}

    test {BITPOS bit=1 with string less than 1 word works} {
        r set str "\x00\x0f\x00"
        r bitpos str 1
    } {12}

    test {BITPOS bit=0 starting at unaligned address} {
        r set str "\xff\xf0\x00"
        r bitpos str 0 1
    } {12}

    test {BITPOS bit=1 starting at unaligned address} {
        r set str "\x00\x0f\xff"
        r bitpos str 1 1
    } {12}

    test {BITPOS bit=0 unaligned+full word+reminder} {
        r del str
        r set str "\xff\xff\xff" ; # Prefix
        # Followed by two (or four in 32 bit systems) full words
        r append str "\xff\xff\xff\xff\xff\xff\xff\xff"
        r append str "\xff\xff\xff\xff\xff\xff\xff\xff"
        r append str "\xff\xff\xff\xff\xff\xff\xff\xff"
        # First zero bit.
        r append str "\x0f"
        assert {[r bitpos str 0] == 216}
        assert {[r bitpos str 0 1] == 216}
        assert {[r bitpos str 0 2] == 216}
        assert {[r bitpos str 0 3] == 216}
        assert {[r bitpos str 0 4] == 216}
        assert {[r bitpos str 0 5] == 216}
        assert {[r bitpos str 0 6] == 216}
        assert {[r bitpos str 0 7] == 216}
        assert {[r bitpos str 0 8] == 216}
    }

    test {BITPOS bit=1 unaligned+full word+reminder} {
        r del str
        r set str "\x00\x00\x00" ; # Prefix
        # Followed by two (or four in 32 bit systems) full words
        r append str "\x00\x00\x00\x00\x00\x00\x00\x00"
        r append str "\x00\x00\x00\x00\x00\x00\x00\x00"
        r append str "\x00\x00\x00\x00\x00\x00\x00\x00"
        # First zero bit.
        r append str "\xf0"
        assert {[r bitpos str 1] == 216}
        assert {[r bitpos str 1 1] == 216}
        assert {[r bitpos str 1 2] == 216}
        assert {[r bitpos str 1 3] == 216}
        assert {[r bitpos str 1 4] == 216}
        assert {[r bitpos str 1 5] == 216}
        assert {[r bitpos str 1 6] == 216}
        assert {[r bitpos str 1 7] == 216}
        assert {[r bitpos str 1 8] == 216}
    }

    test {BITPOS bit=1 returns -1 if string is all 0 bits} {
        r set str ""
        for {set j 0} {$j < 20} {incr j} {
            assert {[r bitpos str 1] == -1}
            r append str "\x00"
        }
    }

    test {BITPOS bit=0 works with intervals} {
        r set str "\x00\xff\x00"
        assert {[r bitpos str 0 0 -1] == 0}
        assert {[r bitpos str 0 1 -1] == 16}
        assert {[r bitpos str 0 2 -1] == 16}
        assert {[r bitpos str 0 2 200] == 16}
        assert {[r bitpos str 0 1 1] == -1}
    }

    test {BITPOS bit=1 works with intervals} {
        r set str "\x00\xff\x00"
        assert {[r bitpos str 1 0 -1] == 8}
        assert {[r bitpos str 1 1 -1] == 8}
        assert {[r bitpos str 1 2 -1] == -1}
        assert {[r bitpos str 1 2 200] == -1}
        assert {[r bitpos str 1 1 1] == 8}
    }

    test {BITPOS bit=0 changes behavior if end is given} {
        r set str "\xff\xff\xff"
        assert {[r bitpos str 0] == 24}
        assert {[r bitpos str 0 0] == 24}
        assert {[r bitpos str 0 0 -1] == -1}
    }

    test {BITPOS bit=1 fuzzy testing using SETBIT} {
        r del str
        set max 524288; # 64k
        set first_one_pos -1
        for {set j 0} {$j < 1000} {incr j} {
            assert {[r bitpos str 1] == $first_one_pos}
            set pos [randomInt $max]
            r setbit str $pos 1
            if {$first_one_pos == -1 || $first_one_pos > $pos} {
                # Update the position of the first 1 bit in the array
                # if the bit we set is on the left of the previous one.
                set first_one_pos $pos
            }
        }
    }

    test {BITPOS bit=0 fuzzy testing using SETBIT} {
        set max 524288; # 64k
        set first_zero_pos $max
        r set str [string repeat "\xff" [expr $max/8]]
        for {set j 0} {$j < 1000} {incr j} {
            assert {[r bitpos str 0] == $first_zero_pos}
            set pos [randomInt $max]
            r setbit str $pos 0
            if {$first_zero_pos > $pos} {
                # Update the position of the first 0 bit in the array
                # if the bit we clear is on the left of the previous one.
                set first_zero_pos $pos
            }
        }
    }
}
