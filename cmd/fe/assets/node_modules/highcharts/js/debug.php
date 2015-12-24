<?php
header("HTTP/1.1 200 OK");
header('Content-Type: text/javascript');

/**
 * This file concatenates the part files and returns the result based on the setup in /build.xml
 */
$target = @$_GET['target'];
$partsDir = '';

if ($target == 'highchartsmore') {
	$partsDir = 'parts-more/';

} else if ($target == 'highmaps') {
	$partsDir = '';
} else if ($target == 'highstock') {
	$partsDir = '';
}

if ($target == 'highcharts3d') {
	$partsDir = 'parts-3d/';
}

if ($target) {
	$xml = simplexml_load_file('../build.xml');

	$files = $xml->xpath("/project/target[@name=\"set.properties\"]/filelist[@id=\"$target.files\"]/file");

	$s = "";
	foreach ($files as $file) {
		$s .= file_get_contents($partsDir . $file['name']);
	}
	// Use latest version of canvas-tools
	$s = str_replace(
		'http://code.highcharts.com@product.cdnpath@/@product.version@/modules/canvas-tools.js',
		"http://code.highcharts.com/modules/canvas-tools.js",
		$s
	);
	echo $s;

} else { // mapdata for instance
	echo file_get_contents('http://code.highcharts.com' . $_SERVER['REQUEST_URI']);
}

?>