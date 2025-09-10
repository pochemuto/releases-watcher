-- public.actual_album определение
-- Drop table
-- DROP TABLE public.actual_album;
CREATE TABLE public.actual_album (
	id varchar NOT NULL,
	artist varchar COLLATE "ru-RU-x-icu" NULL,
	name varchar COLLATE "ru-RU-x-icu" NULL,
	"year" int4 NULL,
	kind varchar NULL,
	version_id int4 NOT NULL,
	CONSTRAINT actual_album_pk PRIMARY KEY (id, version_id)
) PARTITION BY LIST (version_id);
-- public.create_actual_album_partition
CREATE OR REPLACE FUNCTION create_actual_album_partition(v int) RETURNS void LANGUAGE plpgsql AS $$ BEGIN EXECUTE format(
		'CREATE TABLE IF NOT EXISTS %I PARTITION OF actual_album FOR VALUES IN (%s)',
		format('actual_album_v%s', v),
		v
	);
END $$;
-- public.album определение
-- Drop table
-- DROP TABLE public.album;
CREATE TABLE public.album (
	artist varchar COLLATE "ru-RU-x-icu" NOT NULL,
	"name" varchar COLLATE "ru-RU-x-icu" NOT NULL,
	CONSTRAINT album_pk PRIMARY KEY (artist, name)
);
-- public."cache" определение
-- Drop table
-- DROP TABLE public."cache";
CREATE TABLE public."cache" (
	entity varchar NOT NULL,
	id varchar NOT NULL,
	value jsonb NULL,
	ts timestamp DEFAULT CURRENT_TIMESTAMP NULL,
	CONSTRAINT cache_pk PRIMARY KEY (entity, id)
);
CREATE TABLE public."excluded_artist" (
	"artist" varchar NOT NULL,
	PRIMARY KEY ("artist")
);
CREATE TABLE "public"."excluded_album" (
	"artist" varchar NOT NULL,
	"album" varchar NOT NULL,
	PRIMARY KEY ("artist", "album")
);
-- public."version" definition
-- Drop table
-- DROP TABLE public."version";
CREATE TABLE public."version" (
	version_id serial4 NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	published bool DEFAULT false NOT NULL,
	CONSTRAINT version_pkey PRIMARY KEY (version_id)
);