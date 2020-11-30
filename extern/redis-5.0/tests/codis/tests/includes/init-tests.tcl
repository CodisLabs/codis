# Initialization tests -- most units will start including this.

test "(init) Restart killed instances" {
    foreach type {redis} {
        foreach_${type}_id id {
            if {[get_instance_attrib $type $id pid] == -1} {
                puts -nonewline "$type/$id "
                flush stdout
                restart_instance $type $id
            }
        }
    }
}

test "(init) All instances are reachable" {
    foreach_redis_id id {
        # Every node should be reachable.
        wait_for_condition 1000 50 {
            ([catch {R $id ping} ping_reply] == 0) &&
            ($ping_reply eq {PONG})
        } else {
            catch {R $id ping} err
            fail "Node #$id keeps replying '$err' to PING."
        }
    }
}

test "(init) Flush old data and reset the server role" {
    set group_count [expr {[llength $::redis_instances]/2}]
    foreach_redis_id id {
        if {$id < $group_count} {
            # Master instance
            R $id replicaof no one
            R $id flushall
        } else {
            # Replica instance
            set ms_id [expr {$id - $group_count}]
            set ms_host [get_instance_attrib redis $ms_id host]
            set ms_port [get_instance_attrib redis $ms_id port]
            R $id replicaof $ms_host $ms_port
        }
    }

    # Wait for all the replicas to sync
    set max_retry 1000
    foreach_redis_id id {
        if {$id < $group_count} {
            continue
        } elseif {$id > $group_count} {
            set max_retry 3
        }
        wait_for_condition $max_retry 50 {
            [RI $id master_link_status] eq {up}
        } else {
            fail "Unable to init the server role."
        }
    }
}
