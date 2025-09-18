@extends('layouts/app.blade.tpl')
@section('title')
    Welcome to Our Website
@endsection
@section('content')
    <section class="py-5">
        <div class="container">
            <h1>Hello, {{ $user.Name }}!</h1>

            @if ($user.IsAdmin)
                <p class="admin">You have administrator privileges</p>
            @else
                <p>Welcome to our website!</p>
            @endif

            <div class="items">
                <h2>Our items</h2>
                <ul>
                    @foreach ($items as $item)
                        <li>
                            <strong>{{ $item.Name }}</strong>
                            <span>{!! $item.Price !!}</span>
                            <span>{!! $item.HTMLContent !!}</span>
                        </li>
                    @endforeach
                </ul>
            </div>
            <h3>Key Features</h3>
            <div class="row g-4">
                <div class="col-md-4">
                    <div class="card h-100">
                        <div class="card-body">
                            <h5 class="card-title">Feature One</h5>
                            <p class="card-text">Description of feature one goes here.It's simple and effective.</p>
                        </div>
                    </div>
                </div>
                <div class="col-md-4">
                    <div class="card h-100">
                        <div class="card-body">
                            <h5 class="card-title">Feature Two</h5>
                            <p class="card-text">Description of feature two goes here.Easy to use and reliable.</p>
                        </div>
                    </div>
                </div>
                <div class="col-md-4">
                    <div class="card h-100">
                        <div class="card-body">
                            <h5 class="card-title">Feature Three</h5>
                            <p class="card-text">Description of feature three goes here.Fast and secure.</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </section>
@endsection
