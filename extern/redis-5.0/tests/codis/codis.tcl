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

proc pick_slot_key {id slot} {
    set res [R $id slotsscan $slot 0]
    set keys [lindex $res 1]
    return [lindex $keys 0]
}

proc generate_hset_args {x y batch} {
    set args {}
    for {set i 0} {$i < $batch} {incr i} {
        lappend args "f_$x" $y
        incr x
        incr y
    }
    return $args
}

proc generate_zadd_args {x y batch} {
    set args {}
    for {set i 0} {$i < $batch} {incr i} {
        lappend args $y "e_$x"
        incr x
        incr y
    }
    return $args
}

proc generate_sadd_args {x y batch} {
    set args {}
    for {set i 0} {$i < $batch} {incr i} {
        lappend args "e_$y"
        incr y
    }
    return $args
}

proc create_complex_keys {id prefix type size {num 1} {start 0} {batch 1}} {
    set max_idx [expr {$start + $num}]
    for {set j $start} {$j < $max_idx} {incr j} {
        set key "$prefix:$j"
        for {set x 0} {$x < $size} {incr x $batch} {
            set y [expr {$j + $x}]
            if {[string compare $type "hash"] == 0} {
                set args [generate_hset_args $x $y $batch]
                R $id hset $key {*}$args
            } elseif {[string compare $type "zset"] == 0} {
                set args [generate_zadd_args $x $y $batch]
                R $id zadd $key {*}$args
            } elseif {[string compare $type "set"] == 0} {
                set args [generate_sadd_args $x $y $batch]
                R $id sadd $key {*}$args
            } elseif {[string compare $type "list"] == 0} {
                set args [generate_sadd_args $x $y $batch]
                R $id lpush $key {*}$args
            } else {
                puts "unsupported type: $type"
                assert {1 == 0}
            }
        }
    }
    return $max_idx
}

proc create_some_pairs {id prefix cnt1 cnt2 small large} {
    # generate some string type k-v pairs by $prefix
    R $id DEBUG populate $cnt1 $prefix

    # generate some complex type k-v pairs by $prefix
    set batch 100;  # how many new elements will be generated in each round
    set start $cnt1
    set start [create_complex_keys $id $prefix "hash" $small $cnt2 $start]
    set start [create_complex_keys $id $prefix "hash" $large $cnt2 $start $batch]
    set start [create_complex_keys $id $prefix "zset" $small $cnt2 $start]
    set start [create_complex_keys $id $prefix "zset" $large $cnt2 $start $batch]
    set start [create_complex_keys $id $prefix "set" $small $cnt2 $start]
    set start [create_complex_keys $id $prefix "set" $large $cnt2 $start $batch]
    set start [create_complex_keys $id $prefix "list" $small $cnt2 $start]
    set total [create_complex_keys $id $prefix "list" $large $cnt2 $start $batch]
    return $total
}

proc sync_migrate_key {src dst tag key} {
    # init the parameters for the migration
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGONE
    } else {
        set cmd SLOTSMGRTONE
    }
    set timeout 10;  # seconds

    # do the migration
    set res [R $src $cmd $dhost $dport $timeout $key]
    return $res
}

proc sync_migrate_slot {src dst tag slot {print 0}} {
    # init the parameters for the migration
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
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
        set res [R $src $cmd $dhost $dport $timeout $slot]
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
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGONE-ASYNC
    } else {
        set cmd SLOTSMGRTONE-ASYNC
    }
    set timeout 10;  # seconds

    # do the migration
    set res [R $src $cmd $dhost $dport $timeout $bulks $bytes {*}$args]
    return $res
}

proc async_migrate_slot {src dst tag bulks bytes slot num {print 0}} {
    # init the parameters for the migration
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
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
        set res [R $src $cmd $dhost $dport $timeout $bulks $bytes $slot $num]
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

proc handle_async_migrate_done {link type reply} {
    puts "AsyncMigrate finished: $type '$reply'"
}

proc trigger_async_migrate_key {src dst tag bulks bytes args} {
    # create a new client for async migration
    set shost [get_instance_attrib redis $src host]
    set sport [get_instance_attrib redis $src port]
    set link [redis $shost $sport]
    $link blocking 0;  # use non-blocking mode

    # init the parameters for the migration
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGONE-ASYNC
    } else {
        set cmd SLOTSMGRTONE-ASYNC
    }
    set timeout 10;  # seconds

    # trigger the migration
    set callback [list handle_async_migrate_done]
    $link $cmd $dhost $dport $timeout $bulks $bytes {*}$args $callback
    puts "AsyncMigrate key([lindex $args 0]){#$src => #$dst} starting..."
}

proc trigger_async_migrate_slot {src dst tag bulks bytes slot num} {
    # create a new client for async migration
    set shost [get_instance_attrib redis $src host]
    set sport [get_instance_attrib redis $src port]
    set link [redis $shost $sport]
    $link blocking 0;  # use non-blocking mode

    # init the parameters for the migration
    set dhost [get_instance_attrib redis $dst host]
    set dport [get_instance_attrib redis $dst port]
    if {$tag == 1} {
        set cmd SLOTSMGRTTAGSLOT-ASYNC
    } else {
        set cmd SLOTSMGRTSLOT-ASYNC
    }
    set timeout 10;  # seconds

    # trigger the migration
    set callback [list handle_async_migrate_done]
    $link $cmd $dhost $dport $timeout $bulks $bytes $slot $num $callback
    puts "AsyncMigrate slot_$slot{#$src => #$dst} starting..."
}

proc migrate_exec_wrapper {id key args} {
    catch {R $id SLOTSMGRT-EXEC-WRAPPER $key {*}$args} e
    return $e
}

proc get_migration_status {id} {
    puts [R $id SLOTSMGRT-ASYNC-STATUS]
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
