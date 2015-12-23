

// Add language
extend(defaultOptions.lang, {
	zoomIn: 'Zoom in',
	zoomOut: 'Zoom out'
});


// Set the default map navigation options
defaultOptions.mapNavigation = {
	buttonOptions: {
		alignTo: 'plotBox',
		align: 'left',
		verticalAlign: 'top',
		x: 0,
		width: 18,
		height: 18,
		style: {
			fontSize: '15px',
			fontWeight: 'bold',
			textAlign: 'center'
		},
		theme: {
			'stroke-width': 1
		}
	},
	buttons: {
		zoomIn: {
			onclick: function () {
				this.mapZoom(0.5);
			},
			text: '+',
			y: 0
		},
		zoomOut: {
			onclick: function () {
				this.mapZoom(2);
			},
			text: '-',
			y: 28
		}
	}
	// enabled: false,
	// enableButtons: null, // inherit from enabled
	// enableTouchZoom: null, // inherit from enabled
	// enableDoubleClickZoom: null, // inherit from enabled
	// enableDoubleClickZoomTo: false
	// enableMouseWheelZoom: null, // inherit from enabled
};

/**
 * Utility for reading SVG paths directly.
 */
Highcharts.splitPath = function (path) {
	var i;

	// Move letters apart
	path = path.replace(/([A-Za-z])/g, ' $1 ');
	// Trim
	path = path.replace(/^\s*/, '').replace(/\s*$/, '');

	// Split on spaces and commas
	path = path.split(/[ ,]+/);

	// Parse numbers
	for (i = 0; i < path.length; i++) {
		if (!/[a-zA-Z]/.test(path[i])) {
			path[i] = parseFloat(path[i]);
		}
	}
	return path;
};

// A placeholder for map definitions
Highcharts.maps = {};





// Create symbols for the zoom buttons
function selectiveRoundedRect(x, y, w, h, rTopLeft, rTopRight, rBottomRight, rBottomLeft) {
	return ['M', x + rTopLeft, y,
        // top side
        'L', x + w - rTopRight, y,
        // top right corner
        'C', x + w - rTopRight / 2, y, x + w, y + rTopRight / 2, x + w, y + rTopRight,
        // right side
        'L', x + w, y + h - rBottomRight,
        // bottom right corner
        'C', x + w, y + h - rBottomRight / 2, x + w - rBottomRight / 2, y + h, x + w - rBottomRight, y + h,
        // bottom side
        'L', x + rBottomLeft, y + h,
        // bottom left corner
        'C', x + rBottomLeft / 2, y + h, x, y + h - rBottomLeft / 2, x, y + h - rBottomLeft,
        // left side
        'L', x, y + rTopLeft,
        // top left corner
        'C', x, y + rTopLeft / 2, x + rTopLeft / 2, y, x + rTopLeft, y,
        'Z'
    ];
}
SVGRenderer.prototype.symbols.topbutton = function (x, y, w, h, attr) {
	return selectiveRoundedRect(x - 1, y - 1, w, h, attr.r, attr.r, 0, 0);
};
SVGRenderer.prototype.symbols.bottombutton = function (x, y, w, h, attr) {
	return selectiveRoundedRect(x - 1, y - 1, w, h, 0, 0, attr.r, attr.r);
};
// The symbol callbacks are generated on the SVGRenderer object in all browsers. Even
// VML browsers need this in order to generate shapes in export. Now share
// them with the VMLRenderer.
if (Renderer === VMLRenderer) {
	each(['topbutton', 'bottombutton'], function (shape) {
		VMLRenderer.prototype.symbols[shape] = SVGRenderer.prototype.symbols[shape];
	});
}


/**
 * A wrapper for Chart with all the default values for a Map
 */
Highcharts.Map = function (options, callback) {

	var hiddenAxis = {
			endOnTick: false,
			gridLineWidth: 0,
			lineWidth: 0,
			minPadding: 0,
			maxPadding: 0,
			startOnTick: false,
			title: null,
			tickPositions: []
		},
		seriesOptions;

	/* For visual testing
	hiddenAxis.gridLineWidth = 1;
	hiddenAxis.gridZIndex = 10;
	hiddenAxis.tickPositions = undefined;
	// */

	// Don't merge the data
	seriesOptions = options.series;
	options.series = null;

	options = merge(
		{
			chart: {
				panning: 'xy',
				type: 'map'
			},
			xAxis: hiddenAxis,
			yAxis: merge(hiddenAxis, { reversed: true })
		},
		options, // user's options

		{ // forced options
			chart: {
				inverted: false,
				alignTicks: false
			}
		}
	);

	options.series = seriesOptions;


	return new Chart(options, callback);
};
