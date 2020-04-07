# Test for the SENTINEL CKQUORUM command

source "../tests/includes/init-tests.tcl"
set num_sentinels [llength $::sentinel_instances]

test "CKQUORUM reports OK and the right amount of Sentinels" {
    foreach_sentinel_id id {
        assert_match "*OK $num_sentinels usable*" [S $id SENTINEL CKQUORUM mymaster]
    }
}

test "CKQUORUM detects quorum cannot be reached" {
    set orig_quorum [expr {$num_sentinels/2+1}]
    S 0 SENTINEL SET mymaster quorum [expr {$num_sentinels+1}]
    catch {[S 0 SENTINEL CKQUORUM mymaster]} err
    assert_match "*NOQUORUM*" $err
    S 0 SENTINEL SET mymaster quorum $orig_quorum
}

test "CKQUORUM detects failover authorization cannot be reached" {
    set orig_quorum [expr {$num_sentinels/2+1}]
    S 0 SENTINEL SET mymaster quorum 1
    kill_instance sentinel 1
    kill_instance sentinel 2
    kill_instance sentinel 3
    after 5000
    catch {[S 0 SENTINEL CKQUORUM mymaster]} err
    assert_match "*NOQUORUM*" $err
    S 0 SENTINEL SET mymaster quorum $orig_quorum
    restart_instance sentinel 1
    restart_instance sentinel 2
    restart_instance sentinel 3
}

