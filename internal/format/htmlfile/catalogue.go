package htmlfile

// The catalogue a page carries when elements is every: every element the HTML
// standard lists, each one at least once, in a page the Nu HTML Checker calls
// conforming.
//
// The list was read from the standard itself on 2026-10-14 - the index "List of
// elements", 115 names - rather than from memory, which would have missed
// search, selectedcontent and the return of menu. The draft of this block went
// through four readers before it came here (docs/HTML-2-2026-10-14.md section
// 11.2): the checker found one error in it, a label around the new select whose
// button is a second thing to label, and passes it now with nothing to say.
// Chromium 152 builds every element, none of them unknown.
//
// One element is left out, and on purpose: script. A page that runs code is a
// security fixture, and those are a separate, opt-in set (decision D3). Nothing
// here reaches a network either - every picture, the sound and the frame are
// written into the page as data, so opening the file fetches nothing.
//
// The text is plain English in ASCII, the ruby included, so this setting and
// characters stay independent of each other: a page can carry every element
// with no letter outside ASCII, and every script with nothing but paragraphs.

const (
	// picture is an 8 by 8 PNG of one colour, 74 bytes, and sound 16 samples of
	// silence as an 8 bit WAV at 8 kHz, 60 bytes. Both were made once and are
	// written here as they are, so the page is the same on every run.
	picture = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEUlEQVR42mNwaDiAFTEMLQkAYvNgAQlPC90AAAAASUVORK5CYII="
	sound   = "data:audio/wav;base64,UklGRjQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YRAAAACAgICAgICAgICAgICAgICA"

	// catalogueHead is what the catalogue puts in the head of a page. base
	// names no address, so it changes nothing a link resolves to, and the icon
	// is empty data so a browser asks nobody for one.
	catalogueHead = `<base target="_self">` + "\n" +
		`<link rel="icon" href="data:,">` + "\n" +
		`<meta name="description" content="Every element of the HTML standard, each one at least once">` + "\n" +
		"<style>\nmark { background: #ffeb3b }\n</style>\n"

	catalogueBody = `<header>
<hgroup><h1>Every element</h1><p>Every element of the HTML standard, each one at least once.</p></hgroup>
<nav><ul><li><a href="#text">Text</a></li><li><a href="#tables">Tables</a></li><li><a href="#forms">Forms</a></li><li><a href="#media">Media</a></li></ul></nav>
<search><form><label>Search <input type="search" name="q"></label> <button type="submit">Find</button></form></search>
</header>
<main>
<article id="text">
<h2>Text</h2>
<p>Plain <a href="#forms">link</a>, <abbr title="HyperText Markup Language">HTML</abbr>, <b>bold</b>, <bdi>name</bdi>, <bdo dir="rtl">reversed</bdo>, <cite>a title</cite>, <code>code()</code>, <data value="42">forty two</data>, <del>removed</del>, <dfn>term</dfn>, <em>stressed</em>, <i>alternate</i>, <ins>added</ins>, <kbd>Ctrl</kbd>, <mark>marked</mark>, <q>quoted</q>, <s>no longer accurate</s>, <samp>output</samp>, <small>fine print</small>, <span>span</span>, <strong>important</strong>, H<sub>2</sub>O, x<sup>2</sup>, <time datetime="2026-10-14">14 October 2026</time>, <u>annotated</u>, <var>n</var>, long<wbr>word, a line<br>break.</p>
<p><ruby>base<rp>(</rp><rt>annotation</rt><rp>)</rp></ruby> and <math><mi>x</mi><mo>=</mo><mn>2</mn></math> and <slot name="note">slot fallback</slot> and <tfg-note>a custom element</tfg-note>.</p>
<h3>Third level</h3>
<h4>Fourth level</h4>
<h5>Fifth level</h5>
<h6>Sixth level</h6>
<blockquote cite="#text"><p>A quotation in a block.</p></blockquote>
<pre><code>preformatted   text
  keeps   its spaces</code></pre>
<hr>
<dl><dt>Term</dt><dd>Its description.</dd></dl>
<ol start="3" reversed><li>three</li><li>two</li></ol>
<menu><li><button type="button">Menu button</button></li></menu>
<address>Example Street 1, Example City</address>
<details open><summary>Summary of the details</summary><p>The details themselves.</p></details>
<dialog open><p>A dialog that is open.</p></dialog>
<section><h3>A section</h3><p>Inside a section.</p></section>
<div>A div.</div>
</article>
<article id="tables">
<h2>Tables</h2>
<table>
<caption>A table with every part</caption>
<colgroup><col><col span="2"></colgroup>
<thead><tr><th scope="col">id</th><th scope="col">field</th><th scope="col">value</th></tr></thead>
<tbody><tr><td rowspan="2">1</td><td>first</td><td>alpha</td></tr><tr><td colspan="2">spans two columns</td></tr></tbody>
<tfoot><tr><td colspan="3">footer row</td></tr></tfoot>
</table>
</article>
<article id="forms">
<h2>Forms</h2>
<form>
<fieldset>
<legend>Every type of input</legend>
<label>hidden <input type="hidden" name="hidden" value="h"></label>
<label>text <input type="text" name="text"></label>
<label>search <input type="search" name="search"></label>
<label>tel <input type="tel" name="tel"></label>
<label>url <input type="url" name="url"></label>
<label>email <input type="email" name="email"></label>
<label>password <input type="password" name="password"></label>
<label>date <input type="date" name="date"></label>
<label>month <input type="month" name="month"></label>
<label>week <input type="week" name="week"></label>
<label>time <input type="time" name="time"></label>
<label>datetime-local <input type="datetime-local" name="datetime"></label>
<label>number <input type="number" name="number"></label>
<label>range <input type="range" name="range"></label>
<label>color <input type="color" name="color"></label>
<label>checkbox <input type="checkbox" name="checkbox"></label>
<label>radio <input type="radio" name="radio"></label>
<label>file <input type="file" name="file"></label>
<input type="submit" value="submit">
<input type="image" src="` + picture + `" alt="image button" width="8" height="8">
<input type="reset" value="reset">
<input type="button" value="button">
</fieldset>
<label>select <select name="select"><optgroup label="Group"><option>one</option><option>two</option></optgroup></select></label>
<label for="custom">custom select</label> <select id="custom" name="custom"><button><selectedcontent></selectedcontent></button><option>first</option><option>second</option></select>
<label>datalist <input name="choice" list="choices"></label><datalist id="choices"><option value="alpha"></option><option value="beta"></option></datalist>
<label>textarea <textarea name="textarea">text</textarea></label>
<output name="output">output</output>
<label>progress <progress value="3" max="10">3 of 10</progress></label>
<label>meter <meter value="0.6">60%</meter></label>
<button type="submit">button</button>
</form>
</article>
<article id="media">
<h2>Media</h2>
<figure><img src="` + picture + `" alt="a picture" width="8" height="8" usemap="#map"><figcaption>A picture with a map.</figcaption></figure>
<map name="map"><area shape="rect" coords="0,0,8,8" href="#media" alt="the whole picture"></map>
<picture><source srcset="` + picture + `" type="image/png"><img src="` + picture + `" alt="a picture with a source" width="8" height="8"></picture>
<audio controls src="` + sound + `">an audio player</audio>
<video controls width="64" height="48" poster="` + picture + `"><source src="data:video/mp4," type="video/mp4"><track kind="captions" src="data:text/vtt,WEBVTT" srclang="en" label="English">a video player</video>
<canvas width="8" height="8">a canvas</canvas>
<svg width="16" height="16" viewBox="0 0 16 16"><rect width="16" height="16" fill="#4080c0"/></svg>
<object data="` + picture + `" type="image/png" width="8" height="8">an object</object>
<embed src="` + picture + `" type="image/png" width="8" height="8">
<iframe srcdoc="&lt;p&gt;A frame with its own document.&lt;/p&gt;" sandbox title="an inline frame"></iframe>
</article>
</main>
<aside><p>An aside.</p></aside>
<template><p>A template, never shown.</p></template>
<noscript><p>Shown only without scripts.</p></noscript>
<footer><p>Page footer.</p></footer>
`
)

// pageOnly is the elements that live only in a page's skeleton or its head,
// which a fragment has neither of.
var pageOnly = []string{"base", "body", "head", "html", "link", "meta", "style", "title"}

// catalogueHeadFor is what the catalogue adds to the head, nothing when there
// is no catalogue or no head.
func (s settings) catalogueHeadFor() string {
	if !s.every || s.shape != structureDocument {
		return ""
	}
	return catalogueHead
}

// catalogueBlocks is what the catalogue adds to the body.
func (s settings) catalogueBlocks() string {
	if !s.every {
		return ""
	}
	return catalogueBody
}
