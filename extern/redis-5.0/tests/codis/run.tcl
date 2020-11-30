# Codis test suite. Copyright (C) 2020 pingfan.spf tuobaye2006@gmail.com
# This software is released under the BSD License. See the COPYING file for
# more information.

cd tests/codis
source codis.tcl
source ../instances.tcl

set ::master_base_port 30000;
set ::replica_base_port 40000;
set ::group_count 3;  # How many groups(master + replica) we use at max.

proc main {} {
    parse_options
    spawn_instance redis $::master_base_port $::group_count 0
    spawn_instance redis $::replica_base_port $::group_count $::group_count {
        "appendonly yes"
    }
    run_tests
    cleanup
    end_tests
}

if {[catch main e]} {
    puts $::errorInfo
    if {$::pause_on_error} pause_on_error
    cleanup
    exit 1
}
