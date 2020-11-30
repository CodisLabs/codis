# Codis sync migration test.

source "../tests/includes/init-tests.tcl"

test "Migrate one untagged key by sync method" {
    # init the env
    set src_id 0
    set dst_id 1
    R $src_id flushall
    R $dst_id flushall
    R $src_id debug populate 2 "test"
    R $src_id lpush ltest a b c d
    R $src_id hset htest h1 v1 h2 v2 h3 v3
    R $src_id zadd ztest 0 a 1 b 2 c 3 d
    R $src_id sadd stest a b c d
    set dig_src [R $src_id debug digest]

    # check the migration of a non-existent key
    assert_equal 0 [sync_migrate_key $src_id $dst_id "none"]

    # check the migration of a string type key
    set dig_vals [R $src_id debug digest-value "test:0" "test:1"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "test:0"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "test:1"]
    assert_equal $dig_vals [R $dst_id debug digest-value "test:0" "test:1"]

    # check the migration of a list type key
    set dig_val [R $src_id debug digest-value "ltest"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "ltest"]
    assert_equal $dig_val [R $dst_id debug digest-value "ltest"]

    # check the migration of a hash type key
    set dig_val [R $src_id debug digest-value "htest"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "htest"]
    assert_equal $dig_val [R $dst_id debug digest-value "htest"]

    # check the migration of a zset type key
    set dig_val [R $src_id debug digest-value "ztest"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "ztest"]
    assert_equal $dig_val [R $dst_id debug digest-value "ztest"]

    # check the migration of a set type key
    set dig_val [R $src_id debug digest-value "stest"]
    assert_equal 1 [sync_migrate_key $src_id $dst_id "stest"]
    assert_equal $dig_val [R $dst_id debug digest-value "stest"]

    # verify the data isn't corrupted or changed after migration
    assert_equal $dig_src [R $dst_id debug digest]
}

