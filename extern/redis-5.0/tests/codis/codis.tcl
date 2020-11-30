# Codis-specific test functions.
#
# Copyright (C) 2020 pingfan.spf tuobaye2006@gmail.com
# This software is released under the BSD License. See the COPYING file for
# more information.

proc sync_migrate_key {src_id dst_id key {tag 0}} {
    set dst_host [get_instance_attrib redis $dst_id host]
    set dst_port [get_instance_attrib redis $dst_id port]
    set timeout 10;  # seconds
    if {$tag == 0} {
        set res [R $src_id SLOTSMGRTONE $dst_host $dst_port $timeout $key]
    } else {
        set res [R $src_id SLOTSMGRTTAGONE $dst_host $dst_port $timeout $key]
    }
    return $res
}

proc sync_migrate_slot {src_id dst_id slot {tag 0}} {
    set dst_host [get_instance_attrib redis $dst_id host]
    set dst_port [get_instance_attrib redis $dst_id port]
    set timeout 10;  # seconds
    if {$tag == 0} {
        set res [R $src_id SLOTSMGRTSLOT $dst_host $dst_port $timeout $key]
    } else {
        set res [R $src_id SLOTSMGRTTAGSLOT$dst_host $dst_port $timeout $slot]
    }
    return $res
}
