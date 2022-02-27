create table if not exists urlshortener
(
	id bigint not null
		constraint shorten_id
			primary key,
	urloriginal varchar(255) not null,
	urlshort varchar(255) not null,
	clicks integer not null,
	create_at timestamp not null,
	update_at timestamp not null
);

/*alter table urlshortener owner to postgres;*/


