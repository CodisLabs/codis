// Utility functions. If the HighchartsAdapter is not defined, adapter is an empty object
// and all the utility functions will be null. In that case they are populated by the
// default adapters below.
var adapterRun,
	inArray,
	each,
	grep,
	offset,
	map,
	addEvent,
	removeEvent,
	fireEvent,
	washMouseEvent,
	animate,
	stop;

/**
 * Helper function to load and extend Highcharts with adapter functionality. 
 * @param  {object|function} adapter - HighchartsAdapter or jQuery
 */
Highcharts.loadAdapter = function (adapter) {
	
	if (adapter) {
		// If jQuery, then load our default jQueryAdapter
		if (adapter.fn && adapter.fn.jquery) {
			adapter = loadJQueryAdapter(adapter);
		}
		// Initialize the adapter.
		if (adapter.init) {
			adapter.init(pathAnim);
			delete adapter.init; // Avoid copying to Highcharts object
		}
		// Extend Highcharts with adapter functionality.
		Highcharts.extend(Highcharts, adapter);

		// Assign values to local functions.
		adapterRun = Highcharts.adapterRun;
		inArray = Highcharts.inArray;
		each = Highcharts.each;
		grep = Highcharts.grep;
		offset = Highcharts.offset;
		map = Highcharts.map;
		addEvent = Highcharts.addEvent;
		removeEvent = Highcharts.removeEvent;
		fireEvent = Highcharts.fireEvent;
		washMouseEvent = Highcharts.washMouseEvent;
		animate = Highcharts.animate;
		stop = Highcharts.stop;
	}
};

// Load adapter if HighchartsAdapter or jQuery is set on the window.
Highcharts.loadAdapter(win.HighchartsAdapter || win.jQuery);
