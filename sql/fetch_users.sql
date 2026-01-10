SELECT --id,
       friend_code,
       display_name_pending,
       display_name,
       display_name_status,
       --api_key_hash,
       --date_time_created_utc,
      -- date_time_active_utc,
       game_version
FROM public."user"
LIMIT 1000;