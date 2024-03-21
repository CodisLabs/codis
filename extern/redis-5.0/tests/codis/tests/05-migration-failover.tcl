# Codis test for failover capabilities when migrating.

source "../tests/includes/init-tests.tcl"

test "The async migration can be continued after dst server switched" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    set slot 1
    R $src debug populate 100 "{key:65}"
    R $src debug populate 100 "{key:459}"
    R $src debug populate 100 "{key:983}"
    R $src debug populate 100 "{key:1017}"
    R $src debug populate 100 "{key:3520}"
    R $src debug populate 100 "{key:4243}"
    R $src debug populate 100 "{key:5825}"
    R $src debug populate 100 "{key:6528}"
    R $src debug populate 100 "{key:7294}"
    R $src debug populate 100 "{key:8468}"
    set prefix "{2J8mrF}"
    set strcnt 100;    # count of the string type keys
    set cpxcnt 1;      # count of each complex type keys
    set small 20;      # size of the small complex key
    set large 100000;  # size of the large complex key
    set total [create_some_pairs $src $prefix $strcnt $cpxcnt $small $large]
    assert_equal OK [R $src slotscheck]
    # record the digest and slot size brfore migration
    set dig_orig [R $src debug digest]
    set bak_size [get_slot_size $src $slot]
    puts ">>> Init the enviroment(slot=$slot,size=$bak_size): OK"

    # trigger the async migration of slot_1
    set tag 1;  # 0 => SLOTSMGRTSLOT-ASYNC, 1 => SLOTSMGRTTAGSLOT-ASYNC
    set maxbulks 200;     # $small < $maxbulks < $large
    set maxbytes 51200;   # 50KB(a small value, make the migration to be slower)
    set numkeys 5000;     # should be much larger than the slot size
    trigger_async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys

    # trigger the master of destination group switching
    kill_instance redis $dst
    set dst [expr {$dst + $::group_count}]
    wait_for_condition 1000 50 {
        [RI $dst role] eq {master}
    } else {
        fail "The master of destination group switched failed!"
    }
    puts ">>> Wait for the master of destination group switched: PASS"

    # trigger the async migration of slot_1 again to new dst server
    trigger_async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys
    wait_for_condition 50 100 {
        [get_slot_size $src $slot] == 0
    } else {
        fail "AsyncMigrate is taking too much time!"
    }
    puts "AsyncMigrate slot_$slot{#$src => #$dst} finished."
    puts ">>> Wait for the async migration to be finished: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $bak_size [get_slot_size $dst $slot]
    assert_equal $dig_orig [R $dst debug digest]
    assert_equal OK [R $dst slotscheck]
    assert_equal OK [R $src slotscheck]
    puts ">>> Verify the data after failover && migration: PASS"
    puts -nonewline ">>> End of the case: "
}

test "The async migration can be continued after src server switched" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 2; R $dst flushall;
    set slot 1
    R $src debug populate 100 "{key:65}"
    R $src debug populate 100 "{key:459}"
    R $src debug populate 100 "{key:983}"
    R $src debug populate 100 "{key:1017}"
    R $src debug populate 100 "{key:3520}"
    R $src debug populate 100 "{key:4243}"
    R $src debug populate 100 "{key:5825}"
    R $src debug populate 100 "{key:6528}"
    R $src debug populate 100 "{key:7294}"
    R $src debug populate 100 "{key:8468}"
    set prefix "{2J8mrF}"
    set strcnt 100;    # count of the string type keys
    set cpxcnt 1;      # count of each complex type keys
    set small 20;      # size of the small complex key
    set large 100000;  # size of the large complex key
    set total [create_some_pairs $src $prefix $strcnt $cpxcnt $small $large]
    assert_equal OK [R $src slotscheck]
    # record the digest and slot size brfore migration
    set dig_orig [R $src debug digest]
    set bak_size [get_slot_size $src $slot]
    puts ">>> Init the enviroment(slot=$slot,size=$bak_size): OK"

    # restart the replica instance of the src master(The data on the src master
    # which genreated by DEBUG POPULATE command can't propagate to its replica,
    # so we should restart the replica, trigger a full resynchronization, after
    # that, the data on the replica will be the same with the src master.)
    set sreplica [expr {$src + $::group_count}]
    kill_instance redis $sreplica
    restart_instance redis $sreplica
    wait_for_condition 1000 50 {
        [RI $sreplica master_link_status] eq {up}
    } else {
        fail "Replica(#$sreplica) is unable to sync with the src master(#$src)"
    }
    assert_equal $bak_size [get_slot_size $sreplica $slot]
    assert_equal OK [R $sreplica slotscheck]
    puts ">>> Restart the replica of src master and wait it up: PASS"

    # trigger the async migration of slot_1
    set tag 1;  # 0 => SLOTSMGRTSLOT-ASYNC, 1 => SLOTSMGRTTAGSLOT-ASYNC
    set maxbulks 200;     # $small < $maxbulks < $large
    set maxbytes 51200;   # 50KB(a small value, make the migration to be slower)
    set numkeys 5000;     # should be much larger than the slot size
    trigger_async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys

    # trigger the master of source group switching
    kill_instance redis $src
    set src $sreplica
    wait_for_condition 1000 50 {
        [RI $src role] eq {master}
    } else {
        fail "The master of source group switched failed!"
    }
    puts ">>> Wait for the master of source group switched: PASS"

    # trigger the async migration of slot_1 again from new src server
    trigger_async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys
    wait_for_condition 50 100 {
        [get_slot_size $src $slot] == 0
    } else {
        fail "AsyncMigrate is taking too much time!"
    }
    puts "AsyncMigrate slot_$slot{#$src => #$dst} finished."
    puts ">>> Wait for the async migration to be finished: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $bak_size [get_slot_size $dst $slot]
    assert_equal $dig_orig [R $dst debug digest]
    assert_equal OK [R $dst slotscheck]
    assert_equal OK [R $src slotscheck]
    puts ">>> Verify the data after failover && migration: PASS"
    puts -nonewline ">>> End of the case: "
}
