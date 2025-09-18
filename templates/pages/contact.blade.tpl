@extends('layouts/app.blade.tpl')
@section('title')
{{ $title }}
@endsection
@section('inner')@endsection
@section('content')
<section class="py-5">
    <div class="container">
        <h1>Contact Us</h1>
        <p>If you have any questions or would like to get in touch, please reach out to us through the following methods:</p>
        <ul>
            <li>Email: contact@mysite.com</li>
            <li>Phone: (123) 456-7890</li>
            <li>Address: 123 Main St, Anytown, USA</li>
        </ul>
        <map name="contactMap">
            <area shape="rect" coords="34,44,270,350" alt="Contact Area" href="mailto:contact@mysite.com">
            <area shape="rect" coords="290,44,550,350" alt="Phone Area" href="tel:+11234567890">
            <area shape="rect" coords="34,350,270,600" alt="Address Area" href="#">
            <area shape="rect" coords="290,350,550,600" alt="Website Area" href="https://www.mysite.com">
        </map>
    </div>
</section>
@endsection