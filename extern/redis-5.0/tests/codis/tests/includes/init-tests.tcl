# Initialization tests -- most units will start including this.

set debug_print 0

proc group_ms_name {gid} {
    return "codis-ut-$gid"
}

test "(init) Restart killed instances" {
    foreach type {redis sentinel} {
        foreach_${type}_id id {
            if {[get_instance_attrib $type $id pid] == -1} {
                puts -nonewline "$type/$id "
                flush stdout
                restart_instance $type $id
            }
        }
    }
}

proc get_ping_reply {type id} {
    if {[string compare $type "redis"] == 0} {
        R $id ping
    } else {
        S $id ping
    }
}

test "(init) All instances are reachable" {
    foreach type {redis sentinel} {
        foreach_${type}_id id {
            # Every redis node should be reachable.
            wait_for_condition 1000 50 {
                ([catch {get_ping_reply $type $id} ping_reply] == 0) &&
                ($ping_reply eq {PONG})
            } else {
                catch {get_ping_reply $type $id} err
                fail "Node $type#$id keeps replying '$err' to PING."
            }
            if {$debug_print == 1} {
                puts "Node $type#$id reply '$ping_reply' to PING."
            }
        }
    }
}

test "(init) Remove old master entries from sentinels" {
    for {set gid 1} {$gid <= $::group_count} {incr gid} {
        foreach_sentinel_id id {
            catch {S $id SENTINEL REMOVE [group_ms_name $gid]}
        }
    }
}

test "(init) Flush old data and reset the role of redis instances" {
    foreach_redis_id id {
        if {$id < $::group_count} {
            # Master instance
            R $id replicaof no one
            R $id flushall
        } else {
            # Replica instance
            set ms_id [expr {$id - $::group_count}]
            R $id replicaof [get_instance_attrib redis $ms_id host] \
                [get_instance_attrib redis $ms_id port]
        }
    }

    # Wait for all the replicas to sync
    set max_retry 1000;  # for 1st replica instance
    foreach_redis_id id {
        if {$id < $::group_count} {
            continue;         # skip all the master instances
        } elseif {$id > $::group_count} {
            set max_retry 3;  # for 2nd,3rd,4th... replica instances
        }
        wait_for_condition $max_retry 50 {
            [RI $id master_link_status] eq {up}
        } else {
            fail "Unable to init the role of redis#$id."
        }
        if {$debug_print == 1} {
            puts "Replica redis#$id is synced with its master."
        }
    }
}

test "(init) Sentinels can start monitoring all master instances" {
    set quorum [expr {$::sentinel_count/2+1}]
    for {set gid 1} {$gid <= $::group_count} {incr gid} {
        set ms_id [expr {$gid - 1}]
        set name [group_ms_name $gid]
        foreach_sentinel_id id {
            S $id SENTINEL MONITOR $name \
                [get_instance_attrib redis $ms_id host] \
                [get_instance_attrib redis $ms_id port] $quorum
        }
        foreach_sentinel_id id {
            assert {[S $id sentinel master $name] ne {}}
            S $id SENTINEL SET $name down-after-milliseconds 2000
            S $id SENTINEL SET $name failover-timeout 20000
            S $id SENTINEL SET $name parallel-syncs 10
        }
    }
}

test "(init) Sentinels can talk with all master instances" {
    for {set gid 1} {$gid <= $::group_count} {incr gid} {
        set name [group_ms_name $gid]
        foreach_sentinel_id id {
            wait_for_condition 1000 50 {
                [catch {S $id SENTINEL GET-MASTER-ADDR-BY-NAME $name}] == 0
            } else {
                fail "Sentinel#$id can't talk with the master($name)."
            }
        }
        if {$debug_print == 1} {
            puts "All sentinels can talk with the master($name)."
        }
    }
}

proc ms_info_item {id name item} {
    dict get [S $id SENTINEL MASTER $name] $item
}

test "(init) Sentinels are able to auto-discover other sentinels" {
    set other_count [expr {$::sentinel_count - 1}]
    for {set gid 1} {$gid <= $::group_count} {incr gid} {
        set name [group_ms_name $gid]
        foreach_sentinel_id id {
            wait_for_condition 1000 50 {
                [ms_info_item $id $name "num-other-sentinels"] == $other_count
            } else {
                fail "At least some sentinel can't detect some other sentinel"
            }
        }
        if {$debug_print == 1} {
            puts "All sentinels can detect others for master($name)."
        }
    }
}

test "(init) Sentinels are able to auto-discover slaves" {
    for {set gid 1} {$gid <= $::group_count} {incr gid} {
        set name [group_ms_name $gid]
        foreach_sentinel_id id {
            wait_for_condition 1000 50 {
                [ms_info_item $id $name "num-slaves"] == 1
            } else {
                fail "At least some sentinel can't detect some slave"
            }
        }
        if {$debug_print == 1} {
            puts "All sentinels can detect all slaves for master($name)."
        }
    }
}
