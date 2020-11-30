# Codis-specific test functions.
#
# Copyright (C) 2020 pingfan.spf tuobaye2006@gmail.com
# This software is released under the BSD License. See the COPYING file for
# more information.

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

proc creat_some_keys {id prefix type {num 1} {start 0}} {
    set max_idx [expr {$start + $num}]
    for {set j $start} {$j < $max_idx} {incr j} {
        set key "$prefix:$j"
        if {[string compare $type "hash"] == 0} {
            R $id hset $key a $j b [expr $j+1] c [expr $j+2]
        } elseif {[string compare $type "zset"] == 0} {
            R $id zadd $key $j a [expr $j+1] b [expr $j+2] c
        } elseif {[string compare $type "set"] == 0} {
            R $id sadd $key a $j b [expr $j+1] c [expr $j+2]
        } elseif {[string compare $type "list"] == 0} {
            R $id lpush $key a $j b [expr $j+1] c [expr $j+2]
        } else {
            puts "unknown type: $type"
            assert {1 == 0}
        }
    }
    return $max_idx
}

proc sync_migrate_key {src dst key {tag 1}} {
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    set timeout 10;  # seconds
    if {$tag == 0} {
        set res [R $src SLOTSMGRTONE $dst_host $dst_port $timeout $key]
    } else {
        set res [R $src SLOTSMGRTTAGONE $dst_host $dst_port $timeout $key]
    }
    return $res
}

proc sync_migrate_slot {src dst slot {tag 1}} {
    # init the parameters for the migration
    set dst_host [get_instance_attrib redis $dst host]
    set dst_port [get_instance_attrib redis $dst port]
    set timeout 10;  # seconds
    if {$tag == 0} {
        set cmd SLOTSMGRTSLOT
    } else {
        set cmd SLOTSMGRTTAGSLOT
    }

    # circularly migrate the slot from $src to $dst
    set round 0
    set succ 0
    while 1 {
        incr round
        set res [R $src $cmd $dst_host $dst_port $timeout $slot]
        incr succ [lindex $res 0]
        set size [lindex $res 1]
        if {$size == 0} break
    }
    set res [list $round $succ]
    return $res
}
