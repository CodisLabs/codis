/**
 * @license
 * Highcharts compatibilty pack for bringing the 2.x look for the print/export buttons back in Highcharts 3.
 * License: MIT
 * Author: Torstein Honsi
 */

(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (Highcharts) {

	var defaultOptions = Highcharts.getOptions(),
		symbols = Highcharts.Renderer.prototype.symbols,
		extend = Highcharts.extend,
		merge = Highcharts.merge;

	// Add language keys
	extend(defaultOptions.lang, {
		exportButtonTitle: 'Export to raster or vector image',
		printButtonTitle: 'Print the chart'
	});

	// The old button look
	defaultOptions.navigation.buttonOptions = merge(defaultOptions.navigation.buttonOptions, {
		
		borderColor: '#B0B0B0',
		height: 20,
		theme: {
			fill: {
				linearGradient: { x1: 0, y1: 0, x2: 0, y2: 1 },
				stops: [
					[0, '#FFF'],
					[1, '#DDD']
				]
			},
			stroke: '#BBB'
		},
		symbolSize: 12,
		symbolStroke: '#A0A0A0',
		symbolStrokeWidth: 1
	});
	
	// Add the buttons to default options
	extend(defaultOptions.exporting.buttons, {
		exportButton: {
			//enabled: true,
			symbol: 'exportIcon',
			x: -10,
			symbolFill: '#A8BF77',
			_id: 'exportButton',
			_titleKey: 'exportButtonTitle',
			menuItems: defaultOptions.exporting.buttons.contextButton.menuItems.splice(2, 4)

		},
		printButton: {
			//enabled: true,
			symbol: 'printIcon',
			x: -36,
			symbolFill: '#B5C9DF',
			_id: 'printButton',
			_titleKey: 'printButtonTitle',
			onclick: function () {
				this.print();
			}
		}
	});
	delete defaultOptions.exporting.buttons.contextButton;

	/**
	 * Crisp for 1px stroke width, which is default. In the future, consider a smarter,
	 * global function.
	 */
	function crisp(arr) {
		var i = arr.length;
		while (i--) {
			if (typeof arr[i] === 'number') {
				arr[i] = Math.round(arr[i]) - 0.5;		
			}
		}
		return arr;
	}

	// Create the export icon
	symbols.exportIcon = function (x, y, width, height) {
		return crisp([
			'M', // the disk
			x, y + width,
			'L',
			x + width, y + height,
			x + width, y + height * 0.8,
			x, y + height * 0.8,
			'Z',
			'M', // the arrow
			x + width * 0.5, y + height * 0.8,
			'L',
			x + width * 0.8, y + height * 0.4,
			x + width * 0.4, y + height * 0.4,
			x + width * 0.4, y,
			x + width * 0.6, y,
			x + width * 0.6, y + height * 0.4,
			x + width * 0.2, y + height * 0.4,
			'Z'
		]);
	};
	// Create the print icon
	symbols.printIcon = function (x, y, width, height) {
		return crisp([
			'M', // the printer
			x, y + height * 0.7,
			'L',
			x + width, y + height * 0.7,
			x + width, y + height * 0.4,
			x, y + height * 0.4,
			'Z',
			'M', // the upper sheet
			x + width * 0.2, y + height * 0.4,
			'L',
			x + width * 0.2, y,
			x + width * 0.8, y,
			x + width * 0.8, y + height * 0.4,
			'Z',
			'M', // the lower sheet
			x + width * 0.2, y + height * 0.7,
			'L',
			x, y + height,
			x + width, y + height,
			x + width * 0.8, y + height * 0.7,
			'Z'
		]);
	};

}));
