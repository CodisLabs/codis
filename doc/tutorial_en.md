## Auto Rebalance

Codis support dynamic slots migration based on RAM usage to balance data distribution.
 
```
$./cconfig slot rebalance
```

Requirements:

* All slots’ status should be `online`, namely no transportation task is running. 
* All server groups must have a master. 
