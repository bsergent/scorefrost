SELECT COALESCE(SUM(sc.score), 0)
    FROM solution s
    JOIN score sc ON s.id = sc.solution_id
    WHERE s.user_id = 'dd01f0e5-5fd9-4f7e-b8ec-0feaeab12526' 
      AND sc.type_id = 'time_ms';