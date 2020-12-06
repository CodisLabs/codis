# Codis-specific test functions.
#
# Copyright (C) 2020 pingfan.spf tuobaye2006@gmail.com
# This software is released under the BSD License. See the COPYING file for
# more information.

proc get_key_type_size {id key} {
    set type [R $id type $key]
    if {[string compare $type "list"] == 0} {
        set tmp [R $id llen $key]
    } elseif {[string compare $type "hash"] == 0} {
        set tmp [R $id hlen $key]
    } elseif {[string compare $type "zset"] == 0} {
        set tmp [R $id zcard $key]
    } elseif {[string compare $type "set"] == 0} {
        set tmp [R $id scard $key]
    } else {
        puts "unsupported type: $type"
        assert {1 == 0}
    }
    set size [lindex $tmp 0]
    set res [list $type $size]
    return $res
}

proc get_key_slot {id key} {
    set res [R $id slotshashkey $key]
    return [lindex $res 0]
}

proc get_slot_size {id slot} {
    set res [R $id slotsinfo $slot 1]
    set slot_info [lindex $res 0]
    set slot_size [lindex $slot_info 1]
    if {[string compare $slot_size ""] == 0} {
        return 0
    }
    return $slot_size
}

proc create_some_magic_pairs {id prefix type size {num 1} {start 0}} {
    set max_idx [expr {$start + $num}]
    for {set j $start} {$j < $max_idx} {incr j} {
        set key "$prefix:$j"
        for {set x 0} {$x < $size} {incr x} {
            set y [expr {$j + $x}]
            if {[string compare $type "hash"] == 0} {
                R $id hset $key "k_$x" $y
            } elseif {[string compare $type "zset"] == 0} {
                R $id zadd $key $y "k_$x"
            } elseif {[string compare $type "set"] == 0} {
                R $id sadd $key "e_$y"
            } elseif {[string compare $type "list"] == 0} {
                R $id lpush $key "e_$y"
            } else {
                puts "unsupported type: $type"
                assert {1 == 0}
            }
        }
    }
    return $max_idx
}

proc sync_migrate_key {src dst tag key} {
    # init the parameters for the migration
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGONE
    } else {
        set cmd SLOTSMGRTONE
    }
    set timeout 10;  # seconds

    # do the migration
    set res [R $src $cmd $dst_host $dst_port $timeout $key]
    return $res
}

proc sync_migrate_slot {src dst tag slot {print 0}} {
    # init the parameters for the migration
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGSLOT
    } else {
        set cmd SLOTSMGRTSLOT
    }
    set timeout 10;  # seconds

    # circularly migrate the keys of the slot from $src to $dst
    set round 0
    set total 0
    while 1 {
        incr round
        set res [R $src $cmd $dst_host $dst_port $timeout $slot]
        set succ [lindex $res 0]
        set size [lindex $res 1]
        if {$print == 1} {
            puts "Round $round: size=$size,succ=$succ"
        }
        incr total $succ
        if {$size == 0} break
    }
    set res [list $round $total]
    return $res
}

proc async_migrate_key {src dst tag bulks bytes args} {
    # init the parameters for the migration
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGONE-ASYNC
    } else {
        set cmd SLOTSMGRTONE-ASYNC
    }
    set timeout 10;  # seconds

    # do the migration
    set res [R $src $cmd $dst_host $dst_port $timeout $bulks $bytes {*}$args]
    return $res
}

proc async_migrate_slot {src dst tag bulks bytes slot num {print 0}} {
    # init the parameters for the migration
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGSLOT-ASYNC
    } else {
        set cmd SLOTSMGRTSLOT-ASYNC
    }
    set timeout 10;  # seconds

    # circularly migrate the keys of the slot from $src to $dst
    set round 0
    set total 0
    while 1 {
        incr round
        set res [R $src $cmd $dst_host $dst_port $timeout $bulks $bytes $slot $num]
        set succ [lindex $res 0]
        set size [lindex $res 1]
        if {$print == 1} {
            puts "Round $round: size=$size,succ=$succ"
        }
        incr total $succ
        if {$size == 0} break
    }
    set res [list $round $total]
    return $res
}

proc test_async_migration_with_invalid_params {src key tag args} {
    if {$key == 1} {
        if {$tag == 1} {
            set cmd SLOTSMGRTTAGONE-ASYNC
        } else {
            set cmd SLOTSMGRTONE-ASYNC
        }
    } else {
        if {$tag == 1} {
            set cmd SLOTSMGRTTAGSLOT-ASYNC
        } else {
            set cmd SLOTSMGRTSLOT-ASYNC
        }
    }
    # set the normal value of the migration parameters
    set dhost "127.0.0.1"
    set dport 10000
    set timeout 10
    set maxbulks 200
    set maxbytes 1024000
    # set the value over range
    set bigv1 65536;       # USHRT_MAX+1
    set bigv2 2147483648;  # INT_MAX+1

    # check invalid dst port
    catch {R $src $cmd $dhost 0 $timeout $maxbulks $maxbytes {*}$args} e
    assert_match {*ERR*invalid*port*} $e
    catch {R $src $cmd $dhost $bigv1 $timeout $maxbulks $maxbytes {*}$args} e
    assert_match {*ERR*invalid*port*} $e
    puts ">>> ($cmd) Checking of invalid port value: PASS"

    # check invalid timeout
    catch {R $src $cmd $dhost $dport -1 $maxbulks $maxbytes {*}$args} e
    assert_match {*ERR*invalid*timeout*} $e
    catch {R $src $cmd $dhost $dport $bigv2 $maxbulks $maxbytes {*}$args} e
    assert_match {*ERR*invalid*timeout*} $e
    puts ">>> ($cmd) Checking of invalid timeout value: PASS"

    # check invalid maxbulks
    catch {R $src $cmd $dhost $dport $timeout -1 $maxbytes {*}$args} e
    assert_match {*ERR*invalid*maxbulks*} $e
    catch {R $src $cmd $dhost $dport $timeout $bigv2 $maxbytes {*}$args} e
    assert_match {*ERR*invalid*maxbulks*} $e
    puts ">>> ($cmd) Checking of invalid maxbulks value: PASS"

    # check invalid maxbytes
    catch {R $src $cmd $dhost $dport $timeout $maxbulks -1 {*}$args} e
    assert_match {*ERR*invalid*maxbytes*} $e
    catch {R $src $cmd $dhost $dport $timeout $maxbulks $bigv2 {*}$args} e
    assert_match {*ERR*invalid*maxbytes*} $e
    puts ">>> ($cmd) Checking of invalid maxbytes value: PASS"

    if {$key == 1} {
        return $cmd
    }
    set slot [lindex $args 0]
    set num [lindex $args 1]

    # check invalid slotId
    catch {R $src $cmd $dhost $dport $timeout $maxbulks $maxbytes -1 $num} e
    assert_match {*ERR*invalid*slot*} $e
    catch {R $src $cmd $dhost $dport $timeout $maxbulks $maxbytes 1024 $num} e
    assert_match {*ERR*invalid*slot*} $e
    puts ">>> ($cmd) Checking of invalid slotId value: PASS"

    # check invalid numkeys
    catch {R $src $cmd $dhost $dport $timeout $maxbulks $maxbytes $slot -1} e
    assert_match {*ERR*invalid*numkeys*} $e
    catch {R $src $cmd $dhost $dport $timeout $maxbulks $maxbytes $slot $bigv2} e
    assert_match {*ERR*invalid*numkeys*} $e
    puts ">>> ($cmd) Checking of invalid numkeys value: PASS"
    return $cmd
}
