
// Add events to the Chart object itself
extend(Chart.prototype, {
	renderMapNavigation: function () {
		var chart = this,
			options = this.options.mapNavigation,
			buttons = options.buttons,
			n,
			button,
			buttonOptions,
			attr,
			states,
			stopEvent = function (e) {
				if (e) {
					if (e.preventDefault) {
						e.preventDefault();
					}
					if (e.stopPropagation) {
						e.stopPropagation();
					}
					e.cancelBubble = true;
				}
			},
			outerHandler = function (e) {
				this.handler.call(chart, e);
				stopEvent(e); // Stop default click event (#4444)
			};

		if (pick(options.enableButtons, options.enabled) && !chart.renderer.forExport) {
			for (n in buttons) {
				if (buttons.hasOwnProperty(n)) {
					buttonOptions = merge(options.buttonOptions, buttons[n]);
					attr = buttonOptions.theme;
					attr.style = merge(buttonOptions.theme.style, buttonOptions.style); // #3203
					states = attr.states;
					button = chart.renderer.button(
							buttonOptions.text,
							0,
							0,
							outerHandler,
							attr,
							states && states.hover,
							states && states.select,
							0,
							n === 'zoomIn' ? 'topbutton' : 'bottombutton'
						)
						.attr({
							width: buttonOptions.width,
							height: buttonOptions.height,
							title: chart.options.lang[n],
							zIndex: 5
						})
						.add();
					button.handler = buttonOptions.onclick;
					button.align(extend(buttonOptions, { width: button.width, height: 2 * button.height }), null, buttonOptions.alignTo);
					addEvent(button.element, 'dblclick', stopEvent); // Stop double click event (#4444)
				}
			}
		}
	},

	/**
	 * Fit an inner box to an outer. If the inner box overflows left or right, align it to the sides of the
	 * outer. If it overflows both sides, fit it within the outer. This is a pattern that occurs more places
	 * in Highcharts, perhaps it should be elevated to a common utility function.
	 */
	fitToBox: function (inner, outer) {
		each([['x', 'width'], ['y', 'height']], function (dim) {
			var pos = dim[0],
				size = dim[1];

			if (inner[pos] + inner[size] > outer[pos] + outer[size]) { // right overflow
				if (inner[size] > outer[size]) { // the general size is greater, fit fully to outer
					inner[size] = outer[size];
					inner[pos] = outer[pos];
				} else { // align right
					inner[pos] = outer[pos] + outer[size] - inner[size];
				}
			}
			if (inner[size] > outer[size]) {
				inner[size] = outer[size];
			}
			if (inner[pos] < outer[pos]) {
				inner[pos] = outer[pos];
			}
		});


		return inner;
	},

	/**
	 * Zoom the map in or out by a certain amount. Less than 1 zooms in, greater than 1 zooms out.
	 */
	mapZoom: function (howMuch, centerXArg, centerYArg, mouseX, mouseY) {
		/*if (this.isMapZooming) {
			this.mapZoomQueue = arguments;
			return;
		}*/

		var chart = this,
			xAxis = chart.xAxis[0],
			xRange = xAxis.max - xAxis.min,
			centerX = pick(centerXArg, xAxis.min + xRange / 2),
			newXRange = xRange * howMuch,
			yAxis = chart.yAxis[0],
			yRange = yAxis.max - yAxis.min,
			centerY = pick(centerYArg, yAxis.min + yRange / 2),
			newYRange = yRange * howMuch,
			fixToX = mouseX ? ((mouseX - xAxis.pos) / xAxis.len) : 0.5,
			fixToY = mouseY ? ((mouseY - yAxis.pos) / yAxis.len) : 0.5,
			newXMin = centerX - newXRange * fixToX,
			newYMin = centerY - newYRange * fixToY,
			newExt = chart.fitToBox({
				x: newXMin,
				y: newYMin,
				width: newXRange,
				height: newYRange
			}, {
				x: xAxis.dataMin,
				y: yAxis.dataMin,
				width: xAxis.dataMax - xAxis.dataMin,
				height: yAxis.dataMax - yAxis.dataMin
			});

		// When mousewheel zooming, fix the point under the mouse
		if (mouseX) {
			xAxis.fixTo = [mouseX - xAxis.pos, centerXArg];
		}
		if (mouseY) {
			yAxis.fixTo = [mouseY - yAxis.pos, centerYArg];
		}

		// Zoom
		if (howMuch !== undefined) {
			xAxis.setExtremes(newExt.x, newExt.x + newExt.width, false);
			yAxis.setExtremes(newExt.y, newExt.y + newExt.height, false);

		// Reset zoom
		} else {
			xAxis.setExtremes(undefined, undefined, false);
			yAxis.setExtremes(undefined, undefined, false);
		}

		// Prevent zooming until this one is finished animating
		/*chart.holdMapZoom = true;
		setTimeout(function () {
			chart.holdMapZoom = false;
		}, 200);*/
		/*delay = animation ? animation.duration || 500 : 0;
		if (delay) {
			chart.isMapZooming = true;
			setTimeout(function () {
				chart.isMapZooming = false;
				if (chart.mapZoomQueue) {
					chart.mapZoom.apply(chart, chart.mapZoomQueue);
				}
				chart.mapZoomQueue = null;
			}, delay);
		}*/

		chart.redraw();
	}
});

/**
 * Extend the Chart.render method to add zooming and panning
 */
wrap(Chart.prototype, 'render', function (proceed) {
	var chart = this,
		mapNavigation = chart.options.mapNavigation;

	// Render the plus and minus buttons. Doing this before the shapes makes getBBox much quicker, at least in Chrome.
	chart.renderMapNavigation();

	proceed.call(chart);

	// Add the double click event
	if (pick(mapNavigation.enableDoubleClickZoom, mapNavigation.enabled) || mapNavigation.enableDoubleClickZoomTo) {
		addEvent(chart.container, 'dblclick', function (e) {
			chart.pointer.onContainerDblClick(e);
		});
	}

	// Add the mousewheel event
	if (pick(mapNavigation.enableMouseWheelZoom, mapNavigation.enabled)) {
		addEvent(chart.container, document.onmousewheel === undefined ? 'DOMMouseScroll' : 'mousewheel', function (e) {
			chart.pointer.onContainerMouseWheel(e);
			return false;
		});
	}
});
