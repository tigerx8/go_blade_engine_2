<!DOCTYPE html>
<html lang="vi">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>@yield("title")</title>
    <style>
      body {
        font-family: Arial, sans-serif;
        margin: 0;
        padding: 20px;
      }
      .container {
        max-width: 1200px;
        margin: 0 auto;
      }
    </style>
  </head>
  <body>
    <div class="container">
      @include('components/header.blade.tpl')
      <main>@yield('content')</main>
      @include('components/footer.blade.tpl')
    </div>
  </body>
</html>
