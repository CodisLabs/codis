CREATE TABLE `codis` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `parent_path` varchar(255) NOT NULL,
  `path` varchar(255) NOT NULL,
  `value` blob NOT NULL,
   PRIMARY KEY (`id`),
   UNIQUE KEY (`path`),
   index(parent_path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8
