start_server {tags {"repl"}} {
    start_server {} {
        test {First server should have role slave after SLAVEOF} {
            r -1 slaveof [srv 0 host] [srv 0 port]
            wait_for_condition 50 100 {
                [s -1 master_link_status] eq {up}
            } else {
                fail "Replication not started."
            }
        }

        if {$::accurate} {set numops 50000} else {set numops 5000}

        test {MASTER and SLAVE consistency with expire} {
            createComplexDataset r $numops useexpire
            after 4000 ;# Make sure everything expired before taking the digest
            r keys *   ;# Force DEL syntesizing to slave
            after 1000 ;# Wait another second. Now everything should be fine.
            if {[r debug digest] ne [r -1 debug digest]} {
                set csv1 [csvdump r]
                set csv2 [csvdump {r -1}]
                set fd [open /tmp/repldump1.txt w]
                puts -nonewline $fd $csv1
                close $fd
                set fd [open /tmp/repldump2.txt w]
                puts -nonewline $fd $csv2
                close $fd
                puts "Master - Replica inconsistency"
                puts "Run diff -u against /tmp/repldump*.txt for more info"
            }
            assert_equal [r debug digest] [r -1 debug digest]
        }

        test {Slave is able to evict keys created in writable slaves} {
            r -1 select 5
            assert {[r -1 dbsize] == 0}
            r -1 config set slave-read-only no
            r -1 set key1 1 ex 5
            r -1 set key2 2 ex 5
            r -1 set key3 3 ex 5
            assert {[r -1 dbsize] == 3}
            after 6000
            r -1 dbsize
        } {0}
    }
}

start_server {tags {"repl"}} {
    start_server {} {
        test {First server should have role slave after SLAVEOF} {
            r -1 slaveof [srv 0 host] [srv 0 port]
            wait_for_condition 50 100 {
                [s -1 master_link_status] eq {up}
            } else {
                fail "Replication not started."
            }
        }

        set numops 20000 ;# Enough to trigger the Script Cache LRU eviction.

        # While we are at it, enable AOF to test it will be consistent as well
        # after the test.
        r config set appendonly yes

        test {MASTER and SLAVE consistency with EVALSHA replication} {
            array set oldsha {}
            for {set j 0} {$j < $numops} {incr j} {
                set key "key:$j"
                # Make sure to create scripts that have different SHA1s
                set script "return redis.call('incr','$key')"
                set sha1 [r eval "return redis.sha1hex(\"$script\")" 0]
                set oldsha($j) $sha1
                r eval $script 0
                set res [r evalsha $sha1 0]
                assert {$res == 2}
                # Additionally call one of the old scripts as well, at random.
                set res [r evalsha $oldsha([randomInt $j]) 0]
                assert {$res > 2}

                # Trigger an AOF rewrite while we are half-way, this also
                # forces the flush of the script cache, and we will cover
                # more code as a result.
                if {$j == $numops / 2} {
                    catch {r bgrewriteaof}
                }
            }

            wait_for_condition 50 100 {
                [r dbsize] == $numops &&
                [r -1 dbsize] == $numops &&
                [r debug digest] eq [r -1 debug digest]
            } else {
                set csv1 [csvdump r]
                set csv2 [csvdump {r -1}]
                set fd [open /tmp/repldump1.txt w]
                puts -nonewline $fd $csv1
                close $fd
                set fd [open /tmp/repldump2.txt w]
                puts -nonewline $fd $csv2
                close $fd
                puts "Master - Replica inconsistency"
                puts "Run diff -u against /tmp/repldump*.txt for more info"
            }

            set old_digest [r debug digest]
            r config set appendonly no
            r debug loadaof
            set new_digest [r debug digest]
            assert {$old_digest eq $new_digest}
        }

        test {SLAVE can reload "lua" AUX RDB fields of duplicated scripts} {
            # Force a Slave full resynchronization
            r debug change-repl-id
            r -1 client kill type master

            # Check that after a full resync the slave can still load
            # correctly the RDB file: such file will contain "lua" AUX
            # sections with scripts already in the memory of the master.

            wait_for_condition 50 100 {
                [s -1 master_link_status] eq {up}
            } else {
                fail "Replication not started."
            }

            wait_for_condition 50 100 {
                [r debug digest] eq [r -1 debug digest]
            } else {
                fail "DEBUG DIGEST mismatch after full SYNC with many scripts"
            }
        }
    }
}
