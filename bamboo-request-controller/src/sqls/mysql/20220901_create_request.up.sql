create table `retry` (
 `id` varchar(26) not null
,`request_id` varchar(26) not null
,`dst_topic` varchar(64) not null
,`parameter` text not null
,`triggered_at` datetime not null
,primary key(`id`)
,unique(`request_id`)
);
