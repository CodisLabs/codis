/**
 * The Pane object allows options that are common to a set of X and Y axes.
 *
 * In the future, this can be extended to basic Highcharts and Highstock.
 */
function Pane(options, chart, firstAxis) {
	this.init(options, chart, firstAxis);
}

// Extend the Pane prototype
extend(Pane.prototype, {

	/**
	 * Initiate the Pane object
	 */
	init: function (options, chart, firstAxis) {
		var pane = this,
			backgroundOption,
			defaultOptions = pane.defaultOptions;

		pane.chart = chart;

		// Set options. Angular charts have a default background (#3318)
		pane.options = options = merge(defaultOptions, chart.angular ? { background: {} } : undefined, options);

		backgroundOption = options.background;

		// To avoid having weighty logic to place, update and remove the backgrounds,
		// push them to the first axis' plot bands and borrow the existing logic there.
		if (backgroundOption) {
			each([].concat(splat(backgroundOption)).reverse(), function (config) {
				var backgroundColor = config.backgroundColor,  // if defined, replace the old one (specific for gradients)
					axisUserOptions = firstAxis.userOptions;
				config = merge(pane.defaultBackgroundOptions, config);
				if (backgroundColor) {
					config.backgroundColor = backgroundColor;
				}
				config.color = config.backgroundColor; // due to naming in plotBands
				firstAxis.options.plotBands.unshift(config);
				axisUserOptions.plotBands = axisUserOptions.plotBands || []; // #3176
				if (axisUserOptions.plotBands !== firstAxis.options.plotBands) {
					axisUserOptions.plotBands.unshift(config);
				}
			});
		}
	},

	/**
	 * The default options object
	 */
	defaultOptions: {
		// background: {conditional},
		center: ['50%', '50%'],
		size: '85%',
		startAngle: 0
		//endAngle: startAngle + 360
	},

	/**
	 * The default background options
	 */
	defaultBackgroundOptions: {
		shape: 'circle',
		borderWidth: 1,
		borderColor: 'silver',
		backgroundColor: {
			linearGradient: { x1: 0, y1: 0, x2: 0, y2: 1 },
			stops: [
				[0, '#FFF'],
				[1, '#DDD']
			]
		},
		from: -Number.MAX_VALUE, // corrected to axis min
		innerRadius: 0,
		to: Number.MAX_VALUE, // corrected to axis max
		outerRadius: '105%'
	}
});
