<!doctype html>
<html>

<head>
    <meta charset="utf-8">
    <title>Fiber Example</title>
</head>

<body>
    <h1>Welcome, {{ .user.Name }}</h1>
    {{ if .user.IsAdmin }}
    <p>You are an admin.</p>
    {{ end }}

    <p>Request Host: {{ ._fiber.Header "Host" }}</p>
    <p>Query foo: {{ ._fiber.Query "foo" }}</p>
    <p>Param id: {{ ._fiber.Param "id" }}</p>
</body>

</html>