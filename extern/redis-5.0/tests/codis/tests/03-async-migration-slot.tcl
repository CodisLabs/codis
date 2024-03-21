# Codis async migration test for a hash slot.

source "../tests/includes/init-tests.tcl"

test "Migrate a hash slot by async method with invalid parameters" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment: OK"

    set key 0;  # 1 means migrate key, while 0 means migrate slot
    set slot 256
    set num 200
    set tag 0;  # 1 means use the cmd with TAG, while 0 means not
    set cmd [test_async_migration_with_invalid_params $src $key $tag $slot $num]
    puts ">>> Checking of $cmd completed."
    set tag 1
    set cmd [test_async_migration_with_invalid_params $src $key $tag $slot $num]
    puts ">>> Checking of $cmd completed."
    puts -nonewline ">>> End of the case: "
}

test "Migrate one static slot(no writing) by async method" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    R $src debug populate 102400;  # almost 100 keys in each slot
    set rand [randomInt 102400]
    set prefix "{test_$rand}"
    set slot [get_key_slot $src $prefix]
    set small 20;   # size of the small complex key
    set large 500;  # size of the large complex key
    set cnt 5;      # number of the keys in each type
    set total [create_some_pairs $src $prefix $cnt $cnt $small $large]
    assert_equal OK [R $src slotscheck]
    # record the digest and slot size brfore migration
    set dig_src [R $src debug digest]
    set bak_size [get_slot_size $src $slot]
    puts ">>> Init the enviroment(slot=$slot,size=$bak_size,prefix=$prefix): OK"

    # set the parameters of the migration
    set maxbulks 200;      # $small < $maxbulks < $large
    set maxbytes 1048576;  # 1MB
    set numkeys 30;        # should be much smaller than the slot size
    set print 1;           # if print the detail in each round or not

    # migrate the slot from $src to $dst by SLOTSMGRTSLOT-ASYNC
    set tag 0;  # 0 => SLOTSMGRTSLOT-ASYNC, 1 => SLOTSMGRTTAGSLOT-ASYNC
    set res [async_migrate_slot $src $dst $tag $maxbulks $maxbytes $slot $numkeys $print]
    set round1 [lindex $res 0]
    assert_equal $bak_size [lindex $res 1];    # succ == original slot size
    set dst_size [get_slot_size $dst $slot]
    assert_equal $bak_size $dst_size;          # all the keys have been moved
    assert_equal 0 [get_slot_size $src $slot]; # nothing has been left on $src
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate slot_$slot{#$src => #$dst} by SLOTSMGRTSLOT-ASYNC: PASS"

    # migrate the slot from $dst to $src by SLOTSMGRTTAGSLOT-ASYNC
    set tag 1;  # 0 => SLOTSMGRTSLOT-ASYNC, 1 => SLOTSMGRTTAGSLOT-ASYNC
    set res [async_migrate_slot $dst $src $tag $maxbulks $maxbytes $slot $numkeys $print]
    assert {[lindex $res 0] < $round1};        # *TAG* cmd should be faster
    assert_equal $bak_size [lindex $res 1];    # succ == original slot size
    set src_size [get_slot_size $src $slot];
    assert_equal $bak_size $src_size;          # all the keys have been moved
    assert_equal 0 [get_slot_size $dst $slot]; # nothing has been left on $dst
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate slot_$slot{#$dst => #$src} by SLOTSMGRTTAGSLOT-ASYNC: PASS"

    # verify the data isn't corrupted or changed after 2 migrations
    assert_equal $dig_src [R $src debug digest]
    puts ">>> Verify the data after migration: PASS"
    puts -nonewline ">>> End of the case: "
}
