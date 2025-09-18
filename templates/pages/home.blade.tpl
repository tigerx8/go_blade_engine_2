@extends('layouts/app.blade.tpl') @section('title') Welcome to Our Website
@endsection @section('content')
<h1>Hello, {{ $user.Name }}!</h1>

@if($user.IsAdmin)
<p class="admin">You have administrator privileges</p>
@else
<p>Welcome to our website!</p>
@endif

<div class="items">
  <h2>Our items</h2>
  <ul>
    @foreach($items as $item)
    <li>
      <strong>{{ $item.Name }}</strong>
      <span>{!! $item.Price !!}</span>
      <span>{!! $item.HTMLContent !!}</span>
    </li>
    @endforeach
  </ul>
</div>
@endsection
