# Codis test for reading && writing when migrating.

source "../tests/includes/init-tests.tcl"

test "Read&&Write a key by wrapper combine with key async migrating" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    set slot 1
    set strcnt 100;    # count of the string type keys
    set cpxcnt 1;      # count of each complex type keys
    set small 20;      # size of the small complex key
    set large 100000;  # size of the large complex key
    set prefix1 "{2J8mrF}";  # can be hashed to slot1
    set total1 [create_some_pairs $src $prefix1 $strcnt $cpxcnt $small $large]
    set prefix2 "{key:65}";  # can be hashed to slot1
    set total2 [create_some_pairs $src $prefix2 $strcnt $cpxcnt $small $large]
    assert_equal OK [R $src slotscheck]
    # record the digest and slot size brfore migration
    set dig_src [R $src debug digest]
    set bak_size [get_slot_size $src $slot]
    puts ">>> Init the enviroment(slot=$slot,size=$bak_size): OK"

    # random pick 2 keys, k2 will be migrated after k1 has been done
    set rand [expr {$total1 - ([randomInt 4] * 2 + 1)}]
    set k1 "$prefix1:$rand"
    set t1 [lindex [get_key_type_size $src $k1] 0]
    set rand [expr {$total2 - ([randomInt 4] * 2 + 1)}]
    set k2 "$prefix2:$rand"
    set t2 [lindex [get_key_type_size $src $k2] 0]

    # read/write the key by wrapper with invalid parameters
    set res [migrate_exec_wrapper $src $k1]
    assert_match {*ERR*wrong*number*arguments*} $res
    set res [migrate_exec_wrapper $src $k1 JOKE $k1]
    assert_match {*ERR*invalid*command*} $res
    set res [migrate_exec_wrapper $src $k1 TYPE $k1 $k2]
    assert_match {*ERR*wrong*number*arguments*} $res
    puts ">>> R&&W k1($t1, $k1) by wrapper with invalid parameters: PASS"

    # trigger the async migration of k1
    set tag 1;  # 0 => SLOTSMGRTONE-ASYNC, 1 => SLOTSMGRTTAGONE-ASYNC
    set maxbulks 200;     # $small < $maxbulks < $large
    set maxbytes 51200;   # 50KB(a small value, make the migration to be slower)
    trigger_async_migrate_key $src $dst $tag $maxbulks $maxbytes $k1

    # read/write k1 by wrapper when migrating k1
    set res [migrate_exec_wrapper $src $k1 TYPE $k1]
    assert_equal 2 [lindex $res 0]
    assert_equal $t1 [lindex $res 1]
    set res [migrate_exec_wrapper $src $k1 EXPIRE $k1 600]
    assert_match {*ERR*being*migrated*} $res
    puts ">>> R&&W k1($t1, $k1) by wrapper when migrating k1: PASS"

    # read/write k2 by wrapper when migrating k1
    set res [migrate_exec_wrapper $src $k2 TYPE $k2]
    assert_equal 2 [lindex $res 0]
    assert_equal $t2 [lindex $res 1]
    set res [migrate_exec_wrapper $src $k2 EXPIRE $k2 600]
    assert_equal 2 [lindex $res 0]
    set res [migrate_exec_wrapper $src $k2 TTL $k2]
    assert_equal 2 [lindex $res 0]
    assert {[lindex $res 1] > 0}
    puts ">>> R&&W k2($t2, $k2) by wrapper when migrating k1: PASS"

    # wait for the async migration to be finished
    set new_size [expr {$bak_size - $total1}]
    wait_for_condition 50 100 {
        [get_slot_size $src $slot] == $new_size
    } else {
        fail "AsyncMigrate key($k1) is taking too much time!"
    }
    puts "AsyncMigrate key($k1){#$src => #$dst} finished."

    # trigger the async migration of k2
    trigger_async_migrate_key $src $dst $tag $maxbulks $maxbytes $k2

    # read/write k1 by wrapper when migrating k2
    set res [migrate_exec_wrapper $src $k1 TYPE $k1]
    assert_match {*ERR*doesn't*exist*} $res
    set res [migrate_exec_wrapper $src $k1 PERSIST $k1]
    assert_match {*ERR*doesn't*exist*} $res
    puts ">>> R&&W k1($t1, $k1) by wrapper when migrating k2: PASS"

    # read/write k2 by wrapper when migrating k2
    set res [migrate_exec_wrapper $src $k2 TYPE $k2]
    assert_equal 2 [lindex $res 0]
    assert_equal $t2 [lindex $res 1]
    set res [migrate_exec_wrapper $src $k2 PERSIST $k2]
    assert_match {*ERR*being*migrated*} $res
    set res [migrate_exec_wrapper $src $k2 TTL $k2]
    assert_equal 2 [lindex $res 0]
    assert {[lindex $res 1] > 0}
    puts ">>> R&&W k2($t2, $k2) by wrapper when migrating k2: PASS"

    # wait for the async migration to be finished
    wait_for_condition 50 100 {
        [get_slot_size $src $slot] == 0
    } else {
        fail "AsyncMigrate key($k2) is taking too much time!"
    }
    puts "AsyncMigrate key($k2){#$src => #$dst} finished."

    # check and remove the ttl on dst server
    assert {[lindex [R $dst TTL $k2] 0] > 0}
    assert_equal 1 [lindex [R $dst PERSIST $k2] 0]
    # verify the data isn't corrupted or changed after migration
    assert_equal $bak_size [get_slot_size $dst $slot]
    assert_equal $dig_src [R $dst debug digest]
    assert_equal OK [R $dst slotscheck]
    assert_equal OK [R $src slotscheck]
    puts ">>> Verify the data after migration: PASS"
    puts -nonewline ">>> End of the case: "
}

test "Read&&Write a key by wrapper combine with slot async migrating" {
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
    set dig_src [R $src debug digest]
    set bak_size [get_slot_size $src $slot]
    puts ">>> Init the enviroment(slot=$slot,size=$bak_size): OK"

    # random pick a key
    set rand [expr {$total - ([randomInt 4] * 2 + 1)}]
    set key "$prefix:$rand"
    set type [lindex [get_key_type_size $src $key] 0]

    # read/write the key by wrapper before migration
    set res [migrate_exec_wrapper $src $key TYPE $key]
    assert_equal 2 [lindex $res 0]
    assert_equal $type [lindex $res 1]
    set res [migrate_exec_wrapper $src $key EXPIRE $key 600]
    assert_equal 2 [lindex $res 0]
    set res [migrate_exec_wrapper $src $key TTL $key]
    assert_equal 2 [lindex $res 0]
    assert {[lindex $res 1] > 0}
    puts ">>> R&&W key($type, $key) by wrapper before migration: PASS"

    # trigger the async migration of slot_1
    set tag 1;  # 0 => SLOTSMGRTSLOT-ASYNC, 1 => SLOTSMGRTTAGSLOT-ASYNC
    set maxbulks 200;     # $small < $maxbulks < $large
    set maxbytes 51200;   # 50KB(a small value, make the migration to be slower)
    set numkeys 5000;     # should be much larger than the slot size
    trigger_async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys

    # read/write the key by wrapper when migrating
    set res [migrate_exec_wrapper $src $key TYPE $key]
    assert_equal 2 [lindex $res 0]
    assert_equal $type [lindex $res 1]
    set res [migrate_exec_wrapper $src $key PERSIST $key]
    assert_match {*ERR*being*migrated*} $res
    set res [migrate_exec_wrapper $src $key TTL $key]
    assert_equal 2 [lindex $res 0]
    assert {[lindex $res 1] > 0}
    puts ">>> R&&W key($type, $key) by wrapper when migrating slot_1: PASS"

    # wait for the async migration to be finished
    wait_for_condition 50 100 {
        [get_slot_size $src $slot] == 0
    } else {
        fail "AsyncMigrate is taking too much time!"
    }
    puts "AsyncMigrate slot_$slot{#$src => #$dst} finished."

    # read/write the key by wrapper after migration
    set res [migrate_exec_wrapper $src $key TYPE $key]
    assert_match {*ERR*doesn't*exist*} $res
    set res [migrate_exec_wrapper $src $key PERSIST $key]
    assert_match {*ERR*doesn't*exist*} $res
    set res [migrate_exec_wrapper $dst $key TTL $key]
    assert_equal 2 [lindex $res 0]
    assert {[lindex $res 1] > 0}
    set res [migrate_exec_wrapper $dst $key PERSIST $key]
    assert_equal 2 [lindex $res 0]
    assert_equal 1 [lindex $res 1]
    puts ">>> R&&W key($type, $key) by wrapper after migration: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $bak_size [get_slot_size $dst $slot]
    assert_equal $dig_src [R $dst debug digest]
    assert_equal OK [R $dst slotscheck]
    assert_equal OK [R $src slotscheck]
    puts ">>> Verify the data after migration: PASS"
    puts -nonewline ">>> End of the case: "
}
