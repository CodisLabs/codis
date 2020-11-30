# Check the basic logic of codis hash slot.
#
# Note! We will use some magic strings which can be hashed into the specified
# slot in the cases below. Here are the magic strings:
# ----------------------
# | MagicString | Slot |
# ----------------------
# |   6qYGdH    |   0  |
# |   2J8mrF    |   1  |
# |   1yoSmd    |   2  |
# |   0Czt10    | 1021 |
# |   4Htdmu    | 1022 |
# |   6jSAJI    | 1023 |
# ----------------------

start_server {tags {"codis"}} {
    test "SLOTSHASHKEY return the correct slot for the given keys" {
        assert_equal {0 1 2} [r slotshashkey 6qYGdH 2J8mrF 1yoSmd]
        assert_equal {1023 1022 1021} [r slotshashkey 6jSAJI 4Htdmu 0Czt10]
    }

    test "SLOTSINFO basics" {
        # init the env
        r flushall;  # avoid the influence of the last case
        r debug populate 3 "test" 3
        assert_equal {3} [r exists "test:0" "test:1" "test:2"]
        assert_equal {866 1012 590} [r slotshashkey "test:0" "test:1" "test:2"]

        # test slotsinfo without any option
        assert_equal {{590 1} {866 1} {1012 1}} [r slotsinfo]

        # test slotsinfo with start option
        assert_equal {{866 1} {1012 1}} [r slotsinfo 800]
        catch {r slotsinfo -1} e1
        assert_match {*ERR*invalid*slot*number*} $e1
        catch {r slotsinfo 1024} e2
        assert_match {*ERR*invalid*slot*number*} $e2

        # test slotsinfo with start and count option
        assert_equal {{866 1}} [r slotsinfo 800 100]
        catch {r slotsinfo 800 -1} e3
        assert_match {*ERR*invalid*slot*count*} $e3

        # test slotsinfo after delete keys
        r del "test:1"
        assert_equal {{590 1} {866 1}} [r slotsinfo]
        r del "test:0" "test:2"
        assert_equal {} [r slotsinfo]
    }

    test "SLOTSINFO can work after SLOTSDEL/FLUSHDB" {
        # init the env
        r flushall;  # avoid the influence of the last case
        r debug populate 10 "{6qYGdH}" 3;  # add 10 tagged keys into slot0
        r debug populate 10 "{2J8mrF}" 3;  # add 10 tagged keys into slot1
        r debug populate 10 "{1yoSmd}" 3;  # add 10 tagged keys into slot2
        r debug populate 10 "{4Htdmu}" 3;  # add 10 tagged keys into slot1022
        r debug populate 10 "{6jSAJI}" 3;  # add 10 tagged keys into slot1023
        assert_equal {{0 10} {1 10} {2 10} {1022 10} {1023 10}} [r slotsinfo]

        # test if the result of slotsinfo is right after slotsdel
        r slotsdel 1022
        assert_equal {{0 10} {1 10} {2 10} {1023 10}} [r slotsinfo]
        r slotsdel 1 2
        assert_equal {{0 10} {1023 10}} [r slotsinfo]

        # test if the result of slotsinfo is right after flushdb
        r flushdb
        assert_equal {} [r slotsinfo]
    }

    test "SLOTSCHECK basics" {
        # init the env
        r flushall;  # avoid the influence of the last case

        # test slotscheck for normal keys
        r debug populate 5000 "test";  # at least 2 keys in each slot
        assert_equal OK [r slotscheck]

        # test slotscheck for mixed normal keys and tagged keys
        r debug populate 111 "{6qYGdH}";  # add some tagged keys into slot0
        r debug populate 222 "{2J8mrF}";  # add some tagged keys into slot1
        r debug populate 333 "{1yoSmd}";  # add some tagged keys into slot2
        assert_equal OK [r slotscheck]

        # test slotscheck after delete keys
        r del "{6qYGdH}:1" "{2J8mrF}:2" "{1yoSmd}:3" "test:1988" "test:818"
        assert_equal OK [r slotscheck]

        # test slotscheck after slotsdel
        r slotsdel [randomInt 1024];  # randomly delete a slot
        assert_equal OK [r slotscheck]

        # test slotscheck for empty db
        r flushdb
        r slotscheck
    } {OK}

    # verify if slotsscan can iterate all the keys from the right slot
    proc verify_slotsscan {slot pattern len {count "None"}} {
        # iterate all the keys from the given slot
        set cursor 0
        set round 0
        set keys {}
        while 1 {
            incr round
            if {[string compare $count "None"] == 0} {
                set res [r slotsscan $slot $cursor]
            } else {
                set res [r slotsscan $slot $cursor COUNT $count]
            }
            set cursor [lindex $res 0]
            set tmp [lindex $res 1]
            lappend keys {*}$tmp
            if {$cursor == 0} break
        }

        # verify if all the keys in the given slot have been iterated
        set keys [lsort -unique $keys]
        assert_equal $len [llength $keys]
        # verify if the key name match the pattern
        foreach k $keys {
            assert_match $pattern $k
        }

        return $round
    }

    test "SLOTSSCAN only iterate the set of keys which in the right slot" {
        # init the env
        r flushall;  # avoid the influence of the last case
        set len0 16
        set len1 32
        set len2 48
        r debug populate $len0 "{6qYGdH}";  # add some tagged keys into slot0
        r debug populate $len1 "{2J8mrF}";  # add some tagged keys into slot1
        r debug populate $len2 "{1yoSmd}";  # add some tagged keys into slot2
        assert_equal {{0 16} {1 32} {2 48}} [r slotsinfo]

        # test slotsscan with invalid argc
        catch {r slotsscan 0} e
        assert_match {*ERR*wrong*number*of*arguments*} $e
        catch {r slotsscan 0 0 0} e
        assert_match {*ERR*wrong*number*of*arguments*} $e
        catch {r slotsscan 0 0 0 0 0} e
        assert_match {*ERR*wrong*number*of*arguments*} $e

        # test slotsscan without count option
        verify_slotsscan 0 "*6qYGdH*" $len0
        verify_slotsscan 1 "*2J8mrF*" $len1

        # test slotsscan with normal count option
        set round1 [verify_slotsscan 2 "*1yoSmd*" $len2 10]
        set round2 [verify_slotsscan 2 "*1yoSmd*" $len2 20]
        assert {$round2 < $round1}

        # test slotsscan with invalid count option
        catch {r slotsscan 2 0 count -1} e
        assert_match {*ERR*syntax*} $e
        catch {r slotsscan 2 0 count a} e
        assert_match {*ERR*not*an*integer*} $e
    }
}
