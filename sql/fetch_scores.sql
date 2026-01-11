SELECT
	sol.date_time_utc AT TIME ZONE 'UTC' AT TIME ZONE 'America/New_York' AS date_time_local,
	u.display_name AS user_display_name,
	CONCAT(sol.level_id, '_', sol.level_version) AS level,
	sol.result,
	st.display_name AS score_type,
	sc.score AS score_value
FROM public.solution AS sol
JOIN public.score AS sc
	ON sol.id = sc.solution_id
JOIN public."user" AS u
	ON sol.user_id = u.id
JOIN public.score_type AS st
	ON sc.type_id = st.id
--WHERE sc.type_id = 'time_ms'
ORDER BY sol.date_time_utc DESC
LIMIT 1000;