# Codis sync migration test.

source "../tests/includes/init-tests.tcl"

test "Migrate one untagged key by sync method" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    R $src debug populate 2 "test"
    R $src lpush ltest a b c d
    R $src hset htest h1 v1 h2 v2 h3 v3
    R $src zadd ztest 0 a 1 b 2 c 3 d
    R $src sadd stest a b c d
    set dig_src [R $src debug digest]
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment: PASS"

    # select the command for migration
    set tag 0;  # 0 means SLOTSMGRTONE, while 1 means SLOTSMGRTTAGONE

    # check the migration of a non-existent key
    assert_equal 0 [sync_migrate_key $src $dst "none" $tag]
    puts ">>> Migrate a non-existent key: PASS"

    # check the migration of a string type key
    set dig_vals [R $src debug digest-value "test:0" "test:1"]
    assert_equal 1 [sync_migrate_key $src $dst "test:0" $tag]
    assert_equal 1 [sync_migrate_key $src $dst "test:1" $tag]
    assert_equal $dig_vals [R $dst debug digest-value "test:0" "test:1"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a string type key: PASS"

    # check the migration of a list type key
    set dig_val [R $src debug digest-value "ltest"]
    assert_equal 1 [sync_migrate_key $src $dst "ltest" $tag]
    assert_equal $dig_val [R $dst debug digest-value "ltest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a list type key: PASS"

    # check the migration of a hash type key
    set dig_val [R $src debug digest-value "htest"]
    assert_equal 1 [sync_migrate_key $src $dst "htest" $tag]
    assert_equal $dig_val [R $dst debug digest-value "htest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a hash type key: PASS"

    # check the migration of a zset type key
    set dig_val [R $src debug digest-value "ztest"]
    assert_equal 1 [sync_migrate_key $src $dst "ztest" $tag]
    assert_equal $dig_val [R $dst debug digest-value "ztest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a zset type key: PASS"

    # check the migration of a set type key
    set dig_val [R $src debug digest-value "stest"]
    assert_equal 1 [sync_migrate_key $src $dst "stest" $tag]
    assert_equal $dig_val [R $dst debug digest-value "stest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a set type key: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $dig_src [R $dst debug digest]
    puts -nonewline ">>> End of the case: "
}

test "Migrate one tagged key by sync method" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    set prefix "{test}"
    set count [randomInt 10]; incr count;  # avoid the bad case: count == 0
    R $src debug populate $count $prefix
    set total $count
    set total [creat_some_keys $src $prefix "hash" $count $total]
    set total [creat_some_keys $src $prefix "zset" $count $total]
    set total [creat_some_keys $src $prefix "set" $count $total]
    set total [creat_some_keys $src $prefix "list" $count $total]
    set dig_src [R $src debug digest]
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment(count=$count,total=$total): PASS"

    # select the command for migration
    set tag 1;  # 0 means SLOTSMGRTONE, while 1 means SLOTSMGRTTAGONE

    # check the migration of a non-existent tagged key
    assert_equal 0 [sync_migrate_key $src $dst "{none}" $tag]
    puts ">>> Migrate a non-existent key: PASS"

    # check the migration of a randomly picked tagged key
    set rand [randomInt $total]
    set key "$prefix:$rand"
    set type [R $src type $key]
    set dig_val [R $src debug digest-value $key]
    assert_equal $total [sync_migrate_key $src $dst $key $tag]
    assert_equal $dig_val [R $dst debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a tagged key($type, $key){#$src => #$dst}: PASS"

    # check the migration of another tagged key with different type
    set j [expr {($rand + $count) % $total}]
    set key "$prefix:$j"
    set type [R $dst type $key]
    set dig_val [R $dst debug digest-value $key]
    assert_equal $total [sync_migrate_key $dst $src $key $tag]
    assert_equal $dig_val [R $src debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a tagged key($type, $key){#$dst => #$src}: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $dig_src [R $src debug digest]
    puts -nonewline ">>> End of the case: "
}

test "Migrate one static slot(no writing) by sync method" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    R $src debug populate 102400;  # almost 100 keys in each slot
    set rand [randomInt 102400]
    set prefix "{test_$rand}"
    set slot [get_key_slot $src $prefix]
    creat_some_keys $src $prefix "hash" 10 0
    creat_some_keys $src $prefix "zset" 10 10
    creat_some_keys $src $prefix "set" 10 20
    creat_some_keys $src $prefix "list" 10 30
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment(slot=$slot,prefix=$prefix): PASS"

    # record the digest and slot size brfore migration
    set dig_src [R $src debug digest]
    set bak_size [get_slot_size $src $slot]

    # migrate the slot from $src to $dst by SLOTSMGRTSLOT
    set tag 0
    set res [sync_migrate_slot $src $dst $slot $tag]
    set round1 [lindex $res 0]
    assert_equal $bak_size [lindex $res 1];    # succ == original slot size
    set dst_size [get_slot_size $dst $slot]
    assert_equal $bak_size $dst_size;          # all the keys have been moved
    assert_equal 0 [get_slot_size $src $slot]; # nothing has been left on $src
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate slot_$slot{#$src => #$dst} by SLOTSMGRTSLOT: PASS"

    # migrate the slot from $dst to $src by SLOTSMGRTTAGSLOT
    set tag 1
    set res [sync_migrate_slot $dst $src $slot $tag]
    assert {[lindex $res 0] < $round1};        # *TAG* cmd should be faster
    assert_equal $bak_size [lindex $res 1];    # succ == original slot size
    set src_size [get_slot_size $src $slot];
    assert_equal $bak_size $src_size;          # all the keys have been moved
    assert_equal 0 [get_slot_size $dst $slot]; # nothing has been left on $dst
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate slot_$slot{#$dst => #$src} by SLOTSMGRTTAGSLOT: PASS"

    # verify the data isn't corrupted or changed after 2 migrations
    assert_equal $dig_src [R $src debug digest]
    puts -nonewline ">>> End of the case: "
}
