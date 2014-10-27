start_server {tags {"other"}} {
    if {$::force_failure} {
        # This is used just for test suite development purposes.
        test {Failing test} {
            format err
        } {ok}
    }

    tags {slow} {
        if {$::accurate} {set iterations 10000} else {set iterations 1000}
        foreach fuzztype {binary alpha compr} {
            test "FUZZ stresser with data model $fuzztype" {
                set err 0
                for {set i 0} {$i < $iterations} {incr i} {
                    set fuzz [randstring 0 512 $fuzztype]
                    r set foo $fuzz
                    set got [r get foo]
                    if {$got ne $fuzz} {
                        set err [list $fuzz $got]
                        break
                    }
                }
                set _ $err
            } {0}
        }
    }

    tags {protocol} {
        test {PIPELINING stresser (also a regression for the old epoll bug)} {
            set fd2 [socket $::host $::port]
            fconfigure $fd2 -encoding binary -translation binary
            puts -nonewline $fd2 "SELECT 9\r\n"
            flush $fd2
            gets $fd2

            for {set i 0} {$i < 100000} {incr i} {
                set q {}
                set val "0000${i}0000"
                append q "SET key:$i $val\r\n"
                puts -nonewline $fd2 $q
                set q {}
                append q "GET key:$i\r\n"
                puts -nonewline $fd2 $q
            }
            flush $fd2

            for {set i 0} {$i < 100000} {incr i} {
                gets $fd2 line
                gets $fd2 count
                set count [string range $count 1 end]
                set val [read $fd2 $count]
                read $fd2 2
            }
            close $fd2
            set _ 1
        } {1}
    }

    test {APPEND basics} {
        r del foo
        list [r append foo bar] [r get foo] \
             [r append foo 100] [r get foo]
    } {3 bar 6 bar100}

    test {APPEND basics, integer encoded values} {
        set res {}
        r del foo res
        r append foo 1
        r append foo 2
        lappend res [r get foo]
        r set foo 1
        r append foo 2
        lappend res [r get foo]
    } {12 12}

    test {APPEND fuzzing} {
        set err {}
        foreach type {binary alpha compr} {
            set buf {}
            r del x
            for {set i 0} {$i < 1000} {incr i} {
                set bin [randstring 0 10 $type]
                append buf $bin
                r append x $bin
            }
            if {$buf != [r get x]} {
                set err "Expected '$buf' found '[r get x]'"
                break
            }
        }
        set _ $err
    } {}
}
