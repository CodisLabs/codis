

// Extend the Pointer
extend(Pointer.prototype, {

	/**
	 * The event handler for the doubleclick event
	 */
	onContainerDblClick: function (e) {
		var chart = this.chart;

		e = this.normalize(e);

		if (chart.options.mapNavigation.enableDoubleClickZoomTo) {
			if (chart.pointer.inClass(e.target, 'highcharts-tracker')) {
				chart.hoverPoint.zoomTo();
			}
		} else if (chart.isInsidePlot(e.chartX - chart.plotLeft, e.chartY - chart.plotTop)) {
			chart.mapZoom(
				0.5,
				chart.xAxis[0].toValue(e.chartX),
				chart.yAxis[0].toValue(e.chartY),
				e.chartX,
				e.chartY
			);
		}
	},

	/**
	 * The event handler for the mouse scroll event
	 */
	onContainerMouseWheel: function (e) {
		var chart = this.chart,
			delta;

		e = this.normalize(e);

		// Firefox uses e.detail, WebKit and IE uses wheelDelta
		delta = e.detail || -(e.wheelDelta / 120);
		if (chart.isInsidePlot(e.chartX - chart.plotLeft, e.chartY - chart.plotTop)) {
			chart.mapZoom(
				//delta > 0 ? 2 : 0.5,
				Math.pow(2, delta),
				chart.xAxis[0].toValue(e.chartX),
				chart.yAxis[0].toValue(e.chartY),
				e.chartX,
				e.chartY
			);
		}
	}
});

// Implement the pinchType option
wrap(Pointer.prototype, 'init', function (proceed, chart, options) {

	proceed.call(this, chart, options);

	// Pinch status
	if (pick(options.mapNavigation.enableTouchZoom, options.mapNavigation.enabled)) {
		this.pinchX = this.pinchHor = this.pinchY = this.pinchVert = this.hasZoom = true;
	}
});

// Extend the pinchTranslate method to preserve fixed ratio when zooming
wrap(Pointer.prototype, 'pinchTranslate', function (proceed, pinchDown, touches, transform, selectionMarker, clip, lastValidTouch) {
	var xBigger;
	proceed.call(this, pinchDown, touches, transform, selectionMarker, clip, lastValidTouch);

	// Keep ratio
	if (this.chart.options.chart.type === 'map' && this.hasZoom) {
		xBigger = transform.scaleX > transform.scaleY;
		this.pinchTranslateDirection(
			!xBigger,
			pinchDown,
			touches,
			transform,
			selectionMarker,
			clip,
			lastValidTouch,
			xBigger ? transform.scaleX : transform.scaleY
		);
	}
});

