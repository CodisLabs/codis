## Data Migration

Codis offers a reliable and transparent data migration mechanism, also it’s a killer feature which made Codis distinguished from other static distributed Redis solution, such as Twemproxy.

The minimum data migration unit is `key`, we add some specific actions—such as `SLOTSMGRT`—to Codis to support migration based on `key`, which will send a random record of a slot to another Codis Redis instance each time, after the transportation is confirmed the original record will be removed from slot and return slot’s length. The action is atomically.

For example: migrate data in slot with ID from 0 to 511 to server group 2, `--delay` is the sleep duration after each transportation of record, which is used to limit speed, default value is 0. 

```
$ ./cconfig slot migrate 0 511 2 --delay=10
```

Migration progress is reliable and transparent, data won’t vanish and top layer application won’t terminate service. 

Notice that migration task could be paused, but if there is a paused task, it must be fulfilled before another start(means only one migration task is allowed at the same time). 

## Auto Rebalance

Codis support dynamic slots migration based on RAM usage to balance data distribution.
 
```
$./cconfig slot rebalance
```

Requirements:

* All slots’ status should be `online`, namely no transportation task is running. 
* All server groups must have a master. 
