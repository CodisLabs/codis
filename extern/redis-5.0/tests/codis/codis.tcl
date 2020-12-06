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
