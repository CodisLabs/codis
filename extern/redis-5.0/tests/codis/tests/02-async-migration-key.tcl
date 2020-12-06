# Codis async migration test for a single key.

source "../tests/includes/init-tests.tcl"

test "Migrate a single key by async method with invalid parameters" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment: OK"

    set key 1;  # 1 means migrate key, while 0 means migrate slot
    set tag 0;  # 1 means use the cmd with TAG, while 0 means not
    set cmd [test_async_migration_with_invalid_params $src $key $tag "none"]
    puts ">>> Checking of $cmd completed."
    set tag 1
    set cmd [test_async_migration_with_invalid_params $src $key $tag "{none}"]
    puts ">>> Checking of $cmd completed."
    puts -nonewline ">>> End of the case: "
}

test "Migrate one untagged key by async method in PAYLOAD encoding" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    R $src debug populate 2 "test"
    R $src lpush ltest a b c d
    R $src hset htest f1 v1 f2 v2 f3 v3
    R $src zadd ztest 0 a 1 b 2 c 3 d
    R $src sadd stest a b c d
    set dig_src [R $src debug digest]
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment: OK"

    # set the parameters of the migration
    set tag 0;  # 0 means SLOTSMGRTONE-ASYNC, while 1 means SLOTSMGRTTAGONE-ASYNC
    set maxbulks 200;      # should be much larger than the key size
    set maxbytes 1048576;  # 1MB

    # check the migration of a non-existent key
    assert_equal 0 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "none"]
    puts ">>> Migrate a non-existent key: PASS"

    # check the migration of a string type key
    set dig_vals [R $src debug digest-value "test:0" "test:1"]
    assert_equal 2 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "test:0" "test:1"]
    assert_equal $dig_vals [R $dst debug digest-value "test:0" "test:1"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a string type key: PASS"

    # check the migration of a list type key
    set dig_val [R $src debug digest-value "ltest"]
    assert_equal 1 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "ltest"]
    assert_equal $dig_val [R $dst debug digest-value "ltest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a list type key: PASS"

    # check the migration of a hash type key
    set dig_val [R $src debug digest-value "htest"]
    assert_equal 1 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "htest"]
    assert_equal $dig_val [R $dst debug digest-value "htest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a hash type key: PASS"

    # check the migration of a zset type key
    set dig_val [R $src debug digest-value "ztest"]
    assert_equal 1 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "ztest"]
    assert_equal $dig_val [R $dst debug digest-value "ztest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a zset type key: PASS"

    # check the migration of a set type key
    set dig_val [R $src debug digest-value "stest"]
    assert_equal 1 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "stest"]
    assert_equal $dig_val [R $dst debug digest-value "stest"]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a set  type key: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $dig_src [R $dst debug digest]
    puts -nonewline ">>> End of the case: "
}

test "Migrate one tagged key by async method in PAYLOAD encoding" {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    set prefix "{test}"
    set count [randomInt 10]; incr count;  # avoid the bad case: count == 0
    R $src debug populate $count $prefix
    set ksize 5;  # size of the complex key
    set start $count
    set start [create_some_magic_pairs $src $prefix "hash" $ksize $count $start]
    set start [create_some_magic_pairs $src $prefix "zset" $ksize $count $start]
    set start [create_some_magic_pairs $src $prefix "set" $ksize $count $start]
    set total [create_some_magic_pairs $src $prefix "list" $ksize $count $start]
    set dig_src [R $src debug digest]
    assert_equal OK [R $src slotscheck]
    puts ">>> Init the enviroment(count=$count,total=$total): OK"

    # set the parameters of the migration
    set tag 1;  # 0 means SLOTSMGRTONE-ASYNC, while 1 means SLOTSMGRTTAGONE-ASYNC
    set maxbulks 200;      # should be much larger than the key size($ksize)
    set maxbytes 1048576;  # 1MB

    # check the migration of a non-existent tagged key
    assert_equal 0 [async_migrate_key $src $dst $tag $maxbulks $maxbytes "{none}"]
    puts ">>> Migrate a non-existent key: PASS"

    # check the migration of a randomly picked tagged key
    set rand [randomInt $total]
    set key "$prefix:$rand"
    set type [R $src type $key]
    set dig_val [R $src debug digest-value $key]
    assert_equal $total [async_migrate_key $src $dst $tag $maxbulks $maxbytes $key]
    assert_equal $dig_val [R $dst debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a tagged key($type, $key){#$src => #$dst}: PASS"

    # check the migration of another tagged key with different type
    set j [expr {($rand + $count) % $total}]
    set key "$prefix:$j"
    set type [R $dst type $key]
    set dig_val [R $dst debug digest-value $key]
    assert_equal $total [async_migrate_key $dst $src $tag $maxbulks $maxbytes $key]
    assert_equal $dig_val [R $src debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a tagged key($type, $key){#$dst => #$src}: PASS"

    # verify the data isn't corrupted or changed after migration
    assert_equal $dig_src [R $src debug digest]
    puts -nonewline ">>> End of the case: "
}

proc test_bigkey_async_migration {type} {
    # init the env
    puts "Starting..."
    set src 0; R $src flushall;
    set dst 1; R $dst flushall;
    set ksize 20000
    create_some_magic_pairs $src "{test}" $type $ksize 1 0
    assert_equal OK [R $src slotscheck]
    set key "{test}:0"
    set dig_val [R $src debug digest-value $key]
    set desc [get_key_type_size $src $key]
    puts ">>> Init the enviroment($desc): OK"

    # set the parameters of the migration
    set maxbulks 200;      # should be much smaller than the key size($ksize)
    set maxbytes 1048576;  # 1MB

    # migrate the bigkey by SLOTSMGRTONE-ASYNC
    set tag 0;  # 0 means SLOTSMGRTONE-ASYNC, while 1 means SLOTSMGRTTAGONE-ASYNC
    assert_equal 1 [async_migrate_key $src $dst $tag $maxbulks $maxbytes $key]
    assert_equal $dig_val [R $dst debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a big $type key{#$src => #$dst}: PASS"

    # migrate the bigkey by SLOTSMGRTTAGONE-ASYNC
    set tag 1;  # 0 means SLOTSMGRTONE-ASYNC, while 1 means SLOTSMGRTTAGONE-ASYNC
    assert_equal 1 [async_migrate_key $dst $src $tag $maxbulks $maxbytes $key]
    assert_equal $dig_val [R $src debug digest-value $key]
    assert_equal OK [R $src slotscheck]
    assert_equal OK [R $dst slotscheck]
    puts ">>> Migrate a big $type key{#$dst => #$src}: PASS"
}

test "Migrate a list type bigkey by async method in CHUNKED encoding" {
    test_bigkey_async_migration "list"
    puts -nonewline ">>> End of the case: "
}

test "Migrate a hash type bigkey by async method in CHUNKED encoding" {
    test_bigkey_async_migration "hash"
    puts -nonewline ">>> End of the case: "
}

test "Migrate a zset type bigkey by async method in CHUNKED encoding" {
    test_bigkey_async_migration "zset"
    puts -nonewline ">>> End of the case: "
}

test "Migrate a set type bigkey by async method in CHUNKED encoding" {
    test_bigkey_async_migration "set"
    puts -nonewline ">>> End of the case: "
}
