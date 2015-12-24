


Chart.prototype.callbacks.push(function (chart) {
	var extremes,
		scroller = chart.scroller,
		rangeSelector = chart.rangeSelector;

	function renderScroller() {
		extremes = chart.xAxis[0].getExtremes();
		scroller.render(extremes.min, extremes.max);
	}

	function renderRangeSelector() {
		extremes = chart.xAxis[0].getExtremes();
		if (!isNaN(extremes.min)) {
			rangeSelector.render(extremes.min, extremes.max);
		}
	}

	function afterSetExtremesHandlerScroller(e) {
		if (e.triggerOp !== 'navigator-drag') {
			scroller.render(e.min, e.max);
		}
	}

	function afterSetExtremesHandlerRangeSelector(e) {
		rangeSelector.render(e.min, e.max);
	}

	function destroyEvents() {
		if (scroller) {
			removeEvent(chart.xAxis[0], 'afterSetExtremes', afterSetExtremesHandlerScroller);
		}
		if (rangeSelector) {
			removeEvent(chart, 'resize', renderRangeSelector);
			removeEvent(chart.xAxis[0], 'afterSetExtremes', afterSetExtremesHandlerRangeSelector);
		}
	}

	// initiate the scroller
	if (scroller) {
		// redraw the scroller on setExtremes
		addEvent(chart.xAxis[0], 'afterSetExtremes', afterSetExtremesHandlerScroller);

		// redraw the scroller on chart resize or box resize
		wrap(chart, 'drawChartBox', function (proceed) {
			var isDirtyBox = this.isDirtyBox;
			proceed.call(this);
			if (isDirtyBox) {
				renderScroller();
			}
		});

		// do it now
		renderScroller();
	}
	if (rangeSelector) {
		// redraw the scroller on setExtremes
		addEvent(chart.xAxis[0], 'afterSetExtremes', afterSetExtremesHandlerRangeSelector);

		// redraw the scroller chart resize
		addEvent(chart, 'resize', renderRangeSelector);

		// do it now
		renderRangeSelector();
	}

	// Remove resize/afterSetExtremes at chart destroy
	addEvent(chart, 'destroy', destroyEvents);
});
