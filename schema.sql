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
-- public.local_album определение
-- Drop table
-- DROP TABLE public.local_album;
CREATE TABLE public.local_album (
	artist varchar COLLATE "ru-RU-x-icu" NOT NULL,
	"name" varchar COLLATE "ru-RU-x-icu" NOT NULL,
	version_id int4 NOT NULL,
	CONSTRAINT local_album_pk PRIMARY KEY (version_id, artist, name)
) PARTITION BY LIST (version_id);
-- public."local_version" definition
-- Drop table
-- DROP TABLE public."local_version";
CREATE TABLE public."local_version" (
	version_id serial4 NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	published bool DEFAULT false NOT NULL,
	CONSTRAINT local_version_pkey PRIMARY KEY (version_id)
);
-- public.local_album_published source
CREATE OR REPLACE VIEW public.local_album_published AS
SELECT la.artist,
	la.name,
	la.version_id
FROM local_album la
	JOIN (
		SELECT local_version.version_id
		FROM local_version
		WHERE local_version.published = true
		ORDER BY local_version.version_id DESC
		LIMIT 1
	) v ON la.version_id = v.version_id;
-- public.create_local_album_partition
CREATE OR REPLACE FUNCTION create_local_album_partition(v int) RETURNS void LANGUAGE plpgsql AS $$ BEGIN EXECUTE format(
		'CREATE TABLE IF NOT EXISTS %I PARTITION OF local_album FOR VALUES IN (%s)',
		format('local_album_v%s', v),
		v
	);
END $$;
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
-- public."actual_version" definition
-- Drop table
-- DROP TABLE public."actual_version";
CREATE TABLE public."actual_version" (
	version_id serial4 NOT NULL,
	created_at timestamp DEFAULT CURRENT_TIMESTAMP NOT NULL,
	published bool DEFAULT false NOT NULL,
	CONSTRAINT actual_version_pkey PRIMARY KEY (version_id)
);
-- public.actual_album_published source
CREATE OR REPLACE VIEW public.actual_album_published AS
SELECT aa.id,
	aa.artist,
	aa.name,
	aa.year,
	aa.kind,
	aa.version_id
FROM actual_album aa
	JOIN (
		SELECT actual_version.version_id
		FROM actual_version
		WHERE actual_version.published = true
		ORDER BY actual_version.version_id DESC
		LIMIT 1
	) v ON aa.version_id = v.version_id;