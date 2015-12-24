/* ****************************************************************************
 * Start Scroller code														*
 *****************************************************************************/
var units = [].concat(defaultDataGroupingUnits), // copy
	defaultSeriesType,

	// Finding the min or max of a set of variables where we don't know if they are defined,
	// is a pattern that is repeated several places in Highcharts. Consider making this
	// a global utility method.
	numExt = function (extreme) {
		var numbers = grep(arguments, function (n) {
			return typeof n === 'number';
		});
		if (numbers.length) {
			return Math[extreme].apply(0, numbers);
		}
	};

// add more resolution to units
units[4] = ['day', [1, 2, 3, 4]]; // allow more days
units[5] = ['week', [1, 2, 3]]; // allow more weeks

defaultSeriesType = seriesTypes.areaspline === UNDEFINED ? 'line' : 'areaspline';

extend(defaultOptions, {
	navigator: {
		//enabled: true,
		handles: {
			backgroundColor: '#ebe7e8',
			borderColor: '#b2b1b6'
		},
		height: 40,
		margin: 25,
		maskFill: 'rgba(128,179,236,0.3)',
		maskInside: true,
		outlineColor: '#b2b1b6',
		outlineWidth: 1,
		series: {
			type: defaultSeriesType,
			color: '#4572A7',
			compare: null,
			fillOpacity: 0.05,
			dataGrouping: {
				approximation: 'average',
				enabled: true,
				groupPixelWidth: 2,
				smoothed: true,
				units: units
			},
			dataLabels: {
				enabled: false,
				zIndex: 2 // #1839
			},
			id: PREFIX + 'navigator-series',
			lineColor: null, // Allow color setting while disallowing default candlestick setting (#4602)
			lineWidth: 1,
			marker: {
				enabled: false
			},
			pointRange: 0,
			shadow: false,
			threshold: null
		},
		//top: undefined,
		xAxis: {
			tickWidth: 0,
			lineWidth: 0,
			gridLineColor: '#EEE',
			gridLineWidth: 1,
			tickPixelInterval: 200,
			labels: {
				align: 'left',
				style: {
					color: '#888'
				},
				x: 3,
				y: -4
			},
			crosshair: false
		},
		yAxis: {
			gridLineWidth: 0,
			startOnTick: false,
			endOnTick: false,
			minPadding: 0.1,
			maxPadding: 0.1,
			labels: {
				enabled: false
			},
			crosshair: false,
			title: {
				text: null
			},
			tickWidth: 0
		}
	},
	scrollbar: {
		//enabled: true
		height: isTouchDevice ? 20 : 14,
		barBackgroundColor: '#bfc8d1',
		barBorderRadius: 0,
		barBorderWidth: 1,
		barBorderColor: '#bfc8d1',
		buttonArrowColor: '#666',
		buttonBackgroundColor: '#ebe7e8',
		buttonBorderColor: '#bbb',
		buttonBorderRadius: 0,
		buttonBorderWidth: 1,
		minWidth: 6,
		rifleColor: '#666',
		trackBackgroundColor: '#eeeeee',
		trackBorderColor: '#eeeeee',
		trackBorderWidth: 1,
		// trackBorderRadius: 0
		liveRedraw: hasSVG && !isTouchDevice
	}
});

/**
 * The Scroller class
 * @param {Object} chart
 */
function Scroller(chart) {
	var chartOptions = chart.options,
		navigatorOptions = chartOptions.navigator,
		navigatorEnabled = navigatorOptions.enabled,
		scrollbarOptions = chartOptions.scrollbar,
		scrollbarEnabled = scrollbarOptions.enabled,
		height = navigatorEnabled ? navigatorOptions.height : 0,
		scrollbarHeight = scrollbarEnabled ? scrollbarOptions.height : 0;


	this.handles = [];
	this.scrollbarButtons = [];
	this.elementsToDestroy = []; // Array containing the elements to destroy when Scroller is destroyed

	this.chart = chart;
	this.setBaseSeries();

	this.height = height;
	this.scrollbarHeight = scrollbarHeight;
	this.scrollbarEnabled = scrollbarEnabled;
	this.navigatorEnabled = navigatorEnabled;
	this.navigatorOptions = navigatorOptions;
	this.scrollbarOptions = scrollbarOptions;
	this.outlineHeight = height + scrollbarHeight;

	// Run scroller
	this.init();
}

Scroller.prototype = {
	/**
	 * Draw one of the handles on the side of the zoomed range in the navigator
	 * @param {Number} x The x center for the handle
	 * @param {Number} index 0 for left and 1 for right
	 */
	drawHandle: function (x, index) {
		var scroller = this,
			chart = scroller.chart,
			renderer = chart.renderer,
			elementsToDestroy = scroller.elementsToDestroy,
			handles = scroller.handles,
			handlesOptions = scroller.navigatorOptions.handles,
			attr = {
				fill: handlesOptions.backgroundColor,
				stroke: handlesOptions.borderColor,
				'stroke-width': 1
			},
			tempElem;

		// create the elements
		if (!scroller.rendered) {
			// the group
			handles[index] = renderer.g('navigator-handle-' + ['left', 'right'][index])
				.css({ cursor: 'ew-resize' })
				.attr({ zIndex: 10 - index }) // zIndex = 3 for right handle, 4 for left / 10 - #2908
				.add();

			// the rectangle
			tempElem = renderer.rect(-4.5, 0, 9, 16, 0, 1)
				.attr(attr)
				.add(handles[index]);
			elementsToDestroy.push(tempElem);

			// the rifles
			tempElem = renderer.path([
					'M',
					-1.5, 4,
					'L',
					-1.5, 12,
					'M',
					0.5, 4,
					'L',
					0.5, 12
				]).attr(attr)
				.add(handles[index]);
			elementsToDestroy.push(tempElem);
		}

		// Place it
		handles[index][chart.isResizing ? 'animate' : 'attr']({
			translateX: scroller.scrollerLeft + scroller.scrollbarHeight + parseInt(x, 10),
			translateY: scroller.top + scroller.height / 2 - 8
		});
	},

	/**
	 * Draw the scrollbar buttons with arrows
	 * @param {Number} index 0 is left, 1 is right
	 */
	drawScrollbarButton: function (index) {
		var scroller = this,
			chart = scroller.chart,
			renderer = chart.renderer,
			elementsToDestroy = scroller.elementsToDestroy,
			scrollbarButtons = scroller.scrollbarButtons,
			scrollbarHeight = scroller.scrollbarHeight,
			scrollbarOptions = scroller.scrollbarOptions,
			tempElem;

		if (!scroller.rendered) {
			scrollbarButtons[index] = renderer.g().add(scroller.scrollbarGroup);

			tempElem = renderer.rect(
					-0.5,
					-0.5,
					scrollbarHeight + 1, // +1 to compensate for crispifying in rect method
					scrollbarHeight + 1,
					scrollbarOptions.buttonBorderRadius,
					scrollbarOptions.buttonBorderWidth
				).attr({
					stroke: scrollbarOptions.buttonBorderColor,
					'stroke-width': scrollbarOptions.buttonBorderWidth,
					fill: scrollbarOptions.buttonBackgroundColor
				}).add(scrollbarButtons[index]);
			elementsToDestroy.push(tempElem);

			tempElem = renderer.path([
					'M',
					scrollbarHeight / 2 + (index ? -1 : 1), scrollbarHeight / 2 - 3,
					'L',
					scrollbarHeight / 2 + (index ? -1 : 1), scrollbarHeight / 2 + 3,
					scrollbarHeight / 2 + (index ? 2 : -2), scrollbarHeight / 2
				]).attr({
					fill: scrollbarOptions.buttonArrowColor
				}).add(scrollbarButtons[index]);
			elementsToDestroy.push(tempElem);
		}

		// adjust the right side button to the varying length of the scroll track
		if (index) {
			scrollbarButtons[index].attr({
				translateX: scroller.scrollerWidth - scrollbarHeight
			});
		}
	},

	/**
	 * Render the navigator and scroll bar
	 * @param {Number} min X axis value minimum
	 * @param {Number} max X axis value maximum
	 * @param {Number} pxMin Pixel value minimum
	 * @param {Number} pxMax Pixel value maximum
	 */
	render: function (min, max, pxMin, pxMax) {
		var scroller = this,
			chart = scroller.chart,
			renderer = chart.renderer,
			navigatorLeft,
			navigatorWidth,
			scrollerLeft,
			scrollerWidth,
			scrollbarGroup = scroller.scrollbarGroup,
			navigatorGroup = scroller.navigatorGroup,
			scrollbar = scroller.scrollbar,
			xAxis = scroller.xAxis,
			scrollbarTrack = scroller.scrollbarTrack,
			scrollbarHeight = scroller.scrollbarHeight,
			scrollbarEnabled = scroller.scrollbarEnabled,
			navigatorOptions = scroller.navigatorOptions,
			scrollbarOptions = scroller.scrollbarOptions,
			scrollbarMinWidth = scrollbarOptions.minWidth,
			height = scroller.height,
			top = scroller.top,
			navigatorEnabled = scroller.navigatorEnabled,
			outlineWidth = navigatorOptions.outlineWidth,
			halfOutline = outlineWidth / 2,
			zoomedMin,
			zoomedMax,
			range,
			scrX,
			scrWidth,
			scrollbarPad = 0,
			outlineHeight = scroller.outlineHeight,
			barBorderRadius = scrollbarOptions.barBorderRadius,
			strokeWidth,
			scrollbarStrokeWidth = scrollbarOptions.barBorderWidth,
			centerBarX,
			outlineTop = top + halfOutline,
			verb,
			unionExtremes;

		// Don't render the navigator until we have data (#486, #4202). Don't redraw while moving the handles (#4703).
		if (!defined(min) || isNaN(min) || (scroller.hasDragged && !defined(pxMin))) {
			return;
		}

		scroller.navigatorLeft = navigatorLeft = pick(
			xAxis.left,
			chart.plotLeft + scrollbarHeight // in case of scrollbar only, without navigator
		);
		scroller.navigatorWidth = navigatorWidth = pick(xAxis.len, chart.plotWidth - 2 * scrollbarHeight);
		scroller.scrollerLeft = scrollerLeft = navigatorLeft - scrollbarHeight;
		scroller.scrollerWidth = scrollerWidth = scrollerWidth = navigatorWidth + 2 * scrollbarHeight;

		// Set the scroller x axis extremes to reflect the total. The navigator extremes
		// should always be the extremes of the union of all series in the chart as
		// well as the navigator series.
		if (xAxis.getExtremes) {
			unionExtremes = scroller.getUnionExtremes(true);

			if (unionExtremes && (unionExtremes.dataMin !== xAxis.min || unionExtremes.dataMax !== xAxis.max)) {
				xAxis.setExtremes(unionExtremes.dataMin, unionExtremes.dataMax, true, false);
			}
		}

		// Get the pixel position of the handles
		pxMin = pick(pxMin, xAxis.translate(min));
		pxMax = pick(pxMax, xAxis.translate(max));
		if (isNaN(pxMin) || mathAbs(pxMin) === Infinity) { // Verify (#1851, #2238)
			pxMin = 0;
			pxMax = scrollerWidth;
		}

		// Are we below the minRange? (#2618)
		if (xAxis.translate(pxMax, true) - xAxis.translate(pxMin, true) < chart.xAxis[0].minRange) {
			return;
		}


		// handles are allowed to cross, but never exceed the plot area
		scroller.zoomedMax = mathMin(mathMax(pxMin, pxMax, 0), navigatorWidth);
		scroller.zoomedMin =
			mathMax(scroller.fixedWidth ? scroller.zoomedMax - scroller.fixedWidth : mathMin(pxMin, pxMax), 0);
		scroller.range = scroller.zoomedMax - scroller.zoomedMin;
		zoomedMax = mathRound(scroller.zoomedMax);
		zoomedMin = mathRound(scroller.zoomedMin);
		range = zoomedMax - zoomedMin;



		// on first render, create all elements
		if (!scroller.rendered) {

			if (navigatorEnabled) {

				// draw the navigator group
				scroller.navigatorGroup = navigatorGroup = renderer.g('navigator')
					.attr({
						zIndex: 3
					})
					.add();

				scroller.leftShade = renderer.rect()
					.attr({
						fill: navigatorOptions.maskFill
					}).add(navigatorGroup);

				if (navigatorOptions.maskInside) {
					scroller.leftShade.css({ cursor: 'ew-resize' });
				} else {
					scroller.rightShade = renderer.rect()
						.attr({
							fill: navigatorOptions.maskFill
						}).add(navigatorGroup);
				}


				scroller.outline = renderer.path()
					.attr({
						'stroke-width': outlineWidth,
						stroke: navigatorOptions.outlineColor
					})
					.add(navigatorGroup);
			}

			if (scrollbarEnabled) {

				// draw the scrollbar group
				scroller.scrollbarGroup = scrollbarGroup = renderer.g('scrollbar').add();

				// the scrollbar track
				strokeWidth = scrollbarOptions.trackBorderWidth;
				scroller.scrollbarTrack = scrollbarTrack = renderer.rect().attr({
					x: 0,
					y: -strokeWidth % 2 / 2,
					fill: scrollbarOptions.trackBackgroundColor,
					stroke: scrollbarOptions.trackBorderColor,
					'stroke-width': strokeWidth,
					r: scrollbarOptions.trackBorderRadius || 0,
					height: scrollbarHeight
				}).add(scrollbarGroup);

				// the scrollbar itself
				scroller.scrollbar = scrollbar = renderer.rect()
					.attr({
						y: -scrollbarStrokeWidth % 2 / 2,
						height: scrollbarHeight,
						fill: scrollbarOptions.barBackgroundColor,
						stroke: scrollbarOptions.barBorderColor,
						'stroke-width': scrollbarStrokeWidth,
						r: barBorderRadius
					})
					.add(scrollbarGroup);

				scroller.scrollbarRifles = renderer.path()
					.attr({
						stroke: scrollbarOptions.rifleColor,
						'stroke-width': 1
					})
					.add(scrollbarGroup);
			}
		}

		// place elements
		verb = chart.isResizing ? 'animate' : 'attr';

		if (navigatorEnabled) {
			scroller.leftShade[verb](navigatorOptions.maskInside ? {
				x: navigatorLeft + zoomedMin,
				y: top,
				width: zoomedMax - zoomedMin,
				height: height
			} : {
				x: navigatorLeft,
				y: top,
				width: zoomedMin,
				height: height
			});
			if (scroller.rightShade) {
				scroller.rightShade[verb]({
					x: navigatorLeft + zoomedMax,
					y: top,
					width: navigatorWidth - zoomedMax,
					height: height
				});
			}

			scroller.outline[verb]({ d: [
				M,
				scrollerLeft, outlineTop, // left
				L,
				navigatorLeft + zoomedMin - halfOutline, outlineTop, // upper left of zoomed range
				navigatorLeft + zoomedMin - halfOutline, outlineTop + outlineHeight, // lower left of z.r.
				L,
				navigatorLeft + zoomedMax - halfOutline, outlineTop + outlineHeight, // lower right of z.r.
				L,
				navigatorLeft + zoomedMax - halfOutline, outlineTop, // upper right of z.r.
				scrollerLeft + scrollerWidth, outlineTop // right
			].concat(navigatorOptions.maskInside ? [
				M,
				navigatorLeft + zoomedMin + halfOutline, outlineTop, // upper left of zoomed range
				L,
				navigatorLeft + zoomedMax - halfOutline, outlineTop // upper right of z.r.
			] : []) });
			// draw handles
			scroller.drawHandle(zoomedMin + halfOutline, 0);
			scroller.drawHandle(zoomedMax + halfOutline, 1);
		}

		// draw the scrollbar
		if (scrollbarEnabled && scrollbarGroup) {

			// draw the buttons
			scroller.drawScrollbarButton(0);
			scroller.drawScrollbarButton(1);

			scrollbarGroup[verb]({
				translateX: scrollerLeft,
				translateY: mathRound(outlineTop + height)
			});

			scrollbarTrack[verb]({
				width: scrollerWidth
			});

			// prevent the scrollbar from drawing to small (#1246)
			scrX = scrollbarHeight + zoomedMin;
			scrWidth = range - scrollbarStrokeWidth;
			if (scrWidth < scrollbarMinWidth) {
				scrollbarPad = (scrollbarMinWidth - scrWidth) / 2;
				scrWidth = scrollbarMinWidth;
				scrX -= scrollbarPad;
			}
			scroller.scrollbarPad = scrollbarPad;
			scrollbar[verb]({
				x: mathFloor(scrX) + (scrollbarStrokeWidth % 2 / 2),
				width: scrWidth
			});

			centerBarX = scrollbarHeight + zoomedMin + range / 2 - 0.5;

			scroller.scrollbarRifles
				.attr({
					visibility: range > 12 ? VISIBLE : HIDDEN
				})[verb]({
					d: [
						M,
						centerBarX - 3, scrollbarHeight / 4,
						L,
						centerBarX - 3, 2 * scrollbarHeight / 3,
						M,
						centerBarX, scrollbarHeight / 4,
						L,
						centerBarX, 2 * scrollbarHeight / 3,
						M,
						centerBarX + 3, scrollbarHeight / 4,
						L,
						centerBarX + 3, 2 * scrollbarHeight / 3
					]
				});
		}

		scroller.scrollbarPad = scrollbarPad;
		scroller.rendered = true;
	},

	/**
	 * Set up the mouse and touch events for the navigator and scrollbar
	 */
	addEvents: function () {
		var container = this.chart.container,
			mouseDownHandler = this.mouseDownHandler,
			mouseMoveHandler = this.mouseMoveHandler,
			mouseUpHandler = this.mouseUpHandler,
			_events;

		// Mouse events
		_events = [
			[container, 'mousedown', mouseDownHandler],
			[container, 'mousemove', mouseMoveHandler],
			[document, 'mouseup', mouseUpHandler]
		];

		// Touch events
		if (hasTouch) {
			_events.push(
				[container, 'touchstart', mouseDownHandler],
				[container, 'touchmove', mouseMoveHandler],
				[document, 'touchend', mouseUpHandler]
			);
		}

		// Add them all
		each(_events, function (args) {
			addEvent.apply(null, args);
		});
		this._events = _events;
	},

	/**
	 * Removes the event handlers attached previously with addEvents.
	 */
	removeEvents: function () {

		each(this._events, function (args) {
			removeEvent.apply(null, args);
		});
		this._events = UNDEFINED;
		if (this.navigatorEnabled && this.baseSeries) {
			removeEvent(this.baseSeries, 'updatedData', this.updatedDataHandler);
		}
	},

	/**
	 * Initiate the Scroller object
	 */
	init: function () {
		var scroller = this,
			chart = scroller.chart,
			xAxis,
			yAxis,
			scrollbarHeight = scroller.scrollbarHeight,
			navigatorOptions = scroller.navigatorOptions,
			height = scroller.height,
			top = scroller.top,
			dragOffset,
			baseSeries = scroller.baseSeries;

		/**
		 * Event handler for the mouse down event.
		 */
		scroller.mouseDownHandler = function (e) {
			e = chart.pointer.normalize(e);

			var zoomedMin = scroller.zoomedMin,
				zoomedMax = scroller.zoomedMax,
				top = scroller.top,
				scrollbarHeight = scroller.scrollbarHeight,
				scrollerLeft = scroller.scrollerLeft,
				scrollerWidth = scroller.scrollerWidth,
				navigatorLeft = scroller.navigatorLeft,
				navigatorWidth = scroller.navigatorWidth,
				scrollbarPad = scroller.scrollbarPad,
				range = scroller.range,
				chartX = e.chartX,
				chartY = e.chartY,
				baseXAxis = chart.xAxis[0],
				fixedMax,
				ext,
				handleSensitivity = isTouchDevice ? 10 : 7,
				left,
				isOnNavigator;

			if (chartY > top && chartY < top + height + scrollbarHeight) { // we're vertically inside the navigator
				isOnNavigator = !scroller.scrollbarEnabled || chartY < top + height;

				// grab the left handle
				if (isOnNavigator && math.abs(chartX - zoomedMin - navigatorLeft) < handleSensitivity) {
					scroller.grabbedLeft = true;
					scroller.otherHandlePos = zoomedMax;
					scroller.fixedExtreme = baseXAxis.max;
					chart.fixedRange = null;

				// grab the right handle
				} else if (isOnNavigator && math.abs(chartX - zoomedMax - navigatorLeft) < handleSensitivity) {
					scroller.grabbedRight = true;
					scroller.otherHandlePos = zoomedMin;
					scroller.fixedExtreme = baseXAxis.min;
					chart.fixedRange = null;

				// grab the zoomed range
				} else if (chartX > navigatorLeft + zoomedMin - scrollbarPad && chartX < navigatorLeft + zoomedMax + scrollbarPad) {
					scroller.grabbedCenter = chartX;
					scroller.fixedWidth = range;

					dragOffset = chartX - zoomedMin;


				// shift the range by clicking on shaded areas, scrollbar track or scrollbar buttons
				} else if (chartX > scrollerLeft && chartX < scrollerLeft + scrollerWidth) {

					// Center around the clicked point
					if (isOnNavigator) {
						left = chartX - navigatorLeft - range / 2;

					// Click on scrollbar
					} else {

						// Click left scrollbar button
						if (chartX < navigatorLeft) {
							left = zoomedMin - range * 0.2;

						// Click right scrollbar button
						} else if (chartX > scrollerLeft + scrollerWidth - scrollbarHeight) {
							left = zoomedMin + range * 0.2;

						// Click on scrollbar track, shift the scrollbar by one range
						} else {
							left = chartX < navigatorLeft + zoomedMin ? // on the left
								zoomedMin - range :
								zoomedMax;
						}
					}
					if (left < 0) {
						left = 0;
					} else if (left + range >= navigatorWidth) {
						left = navigatorWidth - range;
						fixedMax = scroller.getUnionExtremes().dataMax; // #2293, #3543
					}
					if (left !== zoomedMin) { // it has actually moved
						scroller.fixedWidth = range; // #1370

						ext = xAxis.toFixedRange(left, left + range, null, fixedMax);
						baseXAxis.setExtremes(
							ext.min,
							ext.max,
							true,
							false,
							{ trigger: 'navigator' }
						);
					}
				}

			}
		};

		/**
		 * Event handler for the mouse move event.
		 */
		scroller.mouseMoveHandler = function (e) {
			var scrollbarHeight = scroller.scrollbarHeight,
				navigatorLeft = scroller.navigatorLeft,
				navigatorWidth = scroller.navigatorWidth,
				scrollerLeft = scroller.scrollerLeft,
				scrollerWidth = scroller.scrollerWidth,
				range = scroller.range,
				chartX,
				hasDragged;

			// In iOS, a mousemove event with e.pageX === 0 is fired when holding the finger
			// down in the center of the scrollbar. This should be ignored.
			if (!e.touches || e.touches[0].pageX !== 0) { // #4696, scrollbar failed on Android

				e = chart.pointer.normalize(e);
				chartX = e.chartX;

				// validation for handle dragging
				if (chartX < navigatorLeft) {
					chartX = navigatorLeft;
				} else if (chartX > scrollerLeft + scrollerWidth - scrollbarHeight) {
					chartX = scrollerLeft + scrollerWidth - scrollbarHeight;
				}

				// drag left handle
				if (scroller.grabbedLeft) {
					hasDragged = true;
					scroller.render(0, 0, chartX - navigatorLeft, scroller.otherHandlePos);

				// drag right handle
				} else if (scroller.grabbedRight) {
					hasDragged = true;
					scroller.render(0, 0, scroller.otherHandlePos, chartX - navigatorLeft);

				// drag scrollbar or open area in navigator
				} else if (scroller.grabbedCenter) {

					hasDragged = true;
					if (chartX < dragOffset) { // outside left
						chartX = dragOffset;
					} else if (chartX > navigatorWidth + dragOffset - range) { // outside right
						chartX = navigatorWidth + dragOffset - range;
					}

					scroller.render(0, 0, chartX - dragOffset, chartX - dragOffset + range);

				}
				if (hasDragged && scroller.scrollbarOptions.liveRedraw) {
					setTimeout(function () {
						scroller.mouseUpHandler(e);
					}, 0);
				}
				scroller.hasDragged = hasDragged;
			}
		};

		/**
		 * Event handler for the mouse up event.
		 */
		scroller.mouseUpHandler = function (e) {
			var ext,
				fixedMin,
				fixedMax;

			if (scroller.hasDragged) {
				// When dragging one handle, make sure the other one doesn't change
				if (scroller.zoomedMin === scroller.otherHandlePos) {
					fixedMin = scroller.fixedExtreme;
				} else if (scroller.zoomedMax === scroller.otherHandlePos) {
					fixedMax = scroller.fixedExtreme;
				}

				ext = xAxis.toFixedRange(scroller.zoomedMin, scroller.zoomedMax, fixedMin, fixedMax);
				if (defined(ext.min)) {
					chart.xAxis[0].setExtremes(
						ext.min,
						ext.max,
						true,
						false,
						{
							trigger: 'navigator',
							triggerOp: 'navigator-drag',
							DOMEvent: e // #1838
						}
					);
				}
			}

			if (e.type !== 'mousemove') {
				scroller.grabbedLeft = scroller.grabbedRight = scroller.grabbedCenter = scroller.fixedWidth =
					scroller.fixedExtreme = scroller.otherHandlePos = scroller.hasDragged = dragOffset = null;
			}

		};



		var xAxisIndex = chart.xAxis.length,
			yAxisIndex = chart.yAxis.length;

		// make room below the chart
		chart.extraBottomMargin = scroller.outlineHeight + navigatorOptions.margin;

		if (scroller.navigatorEnabled) {
			// an x axis is required for scrollbar also
			scroller.xAxis = xAxis = new Axis(chart, merge({
				// inherit base xAxis' break and ordinal options
				breaks: baseSeries && baseSeries.xAxis.options.breaks,
				ordinal: baseSeries && baseSeries.xAxis.options.ordinal
			}, navigatorOptions.xAxis, {
				id: 'navigator-x-axis',
				isX: true,
				type: 'datetime',
				index: xAxisIndex,
				height: height,
				offset: 0,
				offsetLeft: scrollbarHeight,
				offsetRight: -scrollbarHeight,
				keepOrdinalPadding: true, // #2436
				startOnTick: false,
				endOnTick: false,
				minPadding: 0,
				maxPadding: 0,
				zoomEnabled: false
			}));

			scroller.yAxis = yAxis = new Axis(chart, merge(navigatorOptions.yAxis, {
				id: 'navigator-y-axis',
				alignTicks: false,
				height: height,
				offset: 0,
				index: yAxisIndex,
				zoomEnabled: false
			}));

			// If we have a base series, initialize the navigator series
			if (baseSeries || navigatorOptions.series.data) {
				scroller.addBaseSeries();

			// If not, set up an event to listen for added series
			} else if (chart.series.length === 0) {

				wrap(chart, 'redraw', function (proceed, animation) {
					// We've got one, now add it as base and reset chart.redraw
					if (chart.series.length > 0 && !scroller.series) {
						scroller.setBaseSeries();
						chart.redraw = proceed; // reset
					}
					proceed.call(chart, animation);
				});
			}


		// in case of scrollbar only, fake an x axis to get translation
		} else {
			scroller.xAxis = xAxis = {
				translate: function (value, reverse) {
					var axis = chart.xAxis[0],
						ext = axis.getExtremes(),
						scrollTrackWidth = chart.plotWidth - 2 * scrollbarHeight,
						min = numExt('min', axis.options.min, ext.dataMin),
						valueRange = numExt('max', axis.options.max, ext.dataMax) - min;

					return reverse ?
						// from pixel to value
						(value * valueRange / scrollTrackWidth) + min :
						// from value to pixel
						scrollTrackWidth * (value - min) / valueRange;
				},
				toFixedRange: Axis.prototype.toFixedRange
			};
		}


		/**
		 * For stock charts, extend the Chart.getMargins method so that we can set the final top position
		 * of the navigator once the height of the chart, including the legend, is determined. #367.
		 */
		wrap(chart, 'getMargins', function (proceed) {

			var legend = this.legend,
				legendOptions = legend.options;

			proceed.apply(this, [].slice.call(arguments, 1));

			// Compute the top position
			scroller.top = top = scroller.navigatorOptions.top ||
				this.chartHeight - scroller.height - scroller.scrollbarHeight - this.spacing[2] -
						(legendOptions.verticalAlign === 'bottom' && legendOptions.enabled && !legendOptions.floating ?
							legend.legendHeight + pick(legendOptions.margin, 10) : 0);

			if (xAxis && yAxis) { // false if navigator is disabled (#904)

				xAxis.options.top = yAxis.options.top = top;

				xAxis.setAxisSize();
				yAxis.setAxisSize();
			}
		});


		scroller.addEvents();
	},

	/**
	 * Get the union data extremes of the chart - the outer data extremes of the base
	 * X axis and the navigator axis.
	 */
	getUnionExtremes: function (returnFalseOnNoBaseSeries) {
		var baseAxis = this.chart.xAxis[0],
			navAxis = this.xAxis,
			navAxisOptions = navAxis.options,
			baseAxisOptions = baseAxis.options,
			ret;

		if (!returnFalseOnNoBaseSeries || baseAxis.dataMin !== null) {
			ret = {
				dataMin: pick( // #4053
					navAxisOptions && navAxisOptions.min,
					numExt(
						'min',
						baseAxisOptions.min,
						baseAxis.dataMin,
						navAxis.dataMin
					)
				),
				dataMax: pick(
					navAxisOptions && navAxisOptions.max,
					numExt(
						'max',
						baseAxisOptions.max,
						baseAxis.dataMax,
						navAxis.dataMax
					)
				)
			};
		}
		return ret;
	},

	/**
	 * Set the base series. With a bit of modification we should be able to make
	 * this an API method to be called from the outside
	 */
	setBaseSeries: function (baseSeriesOption) {
		var chart = this.chart;

		baseSeriesOption = baseSeriesOption || chart.options.navigator.baseSeries;

		// If we're resetting, remove the existing series
		if (this.series) {
			this.series.remove();
		}

		// Set the new base series
		this.baseSeries = chart.series[baseSeriesOption] ||
			(typeof baseSeriesOption === 'string' && chart.get(baseSeriesOption)) ||
			chart.series[0];

		// When run after render, this.xAxis already exists
		if (this.xAxis) {
			this.addBaseSeries();
		}
	},

	addBaseSeries: function () {
		var baseSeries = this.baseSeries,
			baseOptions = baseSeries ? baseSeries.options : {},
			baseData = baseOptions.data,
			mergedNavSeriesOptions,
			navigatorSeriesOptions = this.navigatorOptions.series,
			navigatorData;

		// remove it to prevent merging one by one
		navigatorData = navigatorSeriesOptions.data;
		this.hasNavigatorData = !!navigatorData;

		// Merge the series options
		mergedNavSeriesOptions = merge(baseOptions, navigatorSeriesOptions, {
			enableMouseTracking: false,
			group: 'nav', // for columns
			padXAxis: false,
			xAxis: 'navigator-x-axis',
			yAxis: 'navigator-y-axis',
			name: 'Navigator',
			showInLegend: false,
			isInternal: true,
			visible: true
		});

		// set the data back
		mergedNavSeriesOptions.data = navigatorData || baseData;

		// add the series
		this.series = this.chart.initSeries(mergedNavSeriesOptions);

		// Respond to updated data in the base series.
		// Abort if lazy-loading data from the server.
		if (baseSeries && this.navigatorOptions.adaptToUpdatedData !== false) {
			addEvent(baseSeries, 'updatedData', this.updatedDataHandler);
			// Survive Series.update()
			baseSeries.userOptions.events = extend(baseSeries.userOptions.event, { updatedData: this.updatedDataHandler });

		}
	},

	updatedDataHandler: function () {
		var scroller = this.chart.scroller,
			baseSeries = scroller.baseSeries,
			baseXAxis = baseSeries.xAxis,
			baseExtremes = baseXAxis.getExtremes(),
			baseMin = baseExtremes.min,
			baseMax = baseExtremes.max,
			baseDataMin = baseExtremes.dataMin,
			baseDataMax = baseExtremes.dataMax,
			range = baseMax - baseMin,
			stickToMin,
			stickToMax,
			newMax,
			newMin,
			doRedraw,
			navigatorSeries = scroller.series,
			navXData = navigatorSeries.xData,
			hasSetExtremes = !!baseXAxis.setExtremes;

		// detect whether to move the range
		stickToMax = baseMax >= navXData[navXData.length - 1] - (this.closestPointRange || 0); // #570
		stickToMin = baseMin <= baseDataMin;

		// set the navigator series data to the new data of the base series
		if (!scroller.hasNavigatorData) {
			navigatorSeries.options.pointStart = baseSeries.xData[0];
			navigatorSeries.setData(baseSeries.options.data, false);
			doRedraw = true;
		}

		// if the zoomed range is already at the min, move it to the right as new data
		// comes in
		if (stickToMin) {
			newMin = baseDataMin;
			newMax = newMin + range;
		}

		// if the zoomed range is already at the max, move it to the right as new data
		// comes in
		if (stickToMax) {
			newMax = baseDataMax;
			if (!stickToMin) { // if stickToMin is true, the new min value is set above
				newMin = mathMax(newMax - range, navigatorSeries.xData[0]);
			}
		}

		// update the extremes
		if (hasSetExtremes && (stickToMin || stickToMax)) {
			if (!isNaN(newMin)) {
				baseXAxis.setExtremes(newMin, newMax, true, false, { trigger: 'updatedData' });
			}

		// if it is not at any edge, just move the scroller window to reflect the new series data
		} else {
			if (doRedraw) {
				this.chart.redraw(false);
			}

			scroller.render(
				mathMax(baseMin, baseDataMin),
				mathMin(baseMax, baseDataMax)
			);
		}
	},

	/**
	 * Destroys allocated elements.
	 */
	destroy: function () {
		var scroller = this;

		// Disconnect events added in addEvents
		scroller.removeEvents();

		// Destroy properties
		each([scroller.xAxis, scroller.yAxis, scroller.leftShade, scroller.rightShade, scroller.outline, scroller.scrollbarTrack, scroller.scrollbarRifles, scroller.scrollbarGroup, scroller.scrollbar], function (prop) {
			if (prop && prop.destroy) {
				prop.destroy();
			}
		});
		scroller.xAxis = scroller.yAxis = scroller.leftShade = scroller.rightShade = scroller.outline = scroller.scrollbarTrack = scroller.scrollbarRifles = scroller.scrollbarGroup = scroller.scrollbar = null;

		// Destroy elements in collection
		each([scroller.scrollbarButtons, scroller.handles, scroller.elementsToDestroy], function (coll) {
			destroyObjectProperties(coll);
		});
	}
};

Highcharts.Scroller = Scroller;


/**
 * For Stock charts, override selection zooming with some special features because
 * X axis zooming is already allowed by the Navigator and Range selector.
 */
wrap(Axis.prototype, 'zoom', function (proceed, newMin, newMax) {
	var chart = this.chart,
		chartOptions = chart.options,
		zoomType = chartOptions.chart.zoomType,
		previousZoom,
		navigator = chartOptions.navigator,
		rangeSelector = chartOptions.rangeSelector,
		ret;

	if (this.isXAxis && ((navigator && navigator.enabled) ||
			(rangeSelector && rangeSelector.enabled))) {

		// For x only zooming, fool the chart.zoom method not to create the zoom button
		// because the property already exists
		if (zoomType === 'x') {
			chart.resetZoomButton = 'blocked';

		// For y only zooming, ignore the X axis completely
		} else if (zoomType === 'y') {
			ret = false;

		// For xy zooming, record the state of the zoom before zoom selection, then when
		// the reset button is pressed, revert to this state
		} else if (zoomType === 'xy') {
			previousZoom = this.previousZoom;
			if (defined(newMin)) {
				this.previousZoom = [this.min, this.max];
			} else if (previousZoom) {
				newMin = previousZoom[0];
				newMax = previousZoom[1];
				delete this.previousZoom;
			}
		}

	}
	return ret !== UNDEFINED ? ret : proceed.call(this, newMin, newMax);
});

// Initialize scroller for stock charts
wrap(Chart.prototype, 'init', function (proceed, options, callback) {

	addEvent(this, 'beforeRender', function () {
		var options = this.options;
		if (options.navigator.enabled || options.scrollbar.enabled) {
			this.scroller = new Scroller(this);
		}
	});

	proceed.call(this, options, callback);

});

// Pick up badly formatted point options to addPoint
wrap(Series.prototype, 'addPoint', function (proceed, options, redraw, shift, animation) {
	var turboThreshold = this.options.turboThreshold;
	if (turboThreshold && this.xData.length > turboThreshold && isObject(options) && !isArray(options) && this.chart.scroller) {
		error(20, true);
	}
	proceed.call(this, options, redraw, shift, animation);
});

/* ****************************************************************************
 * End Scroller code														  *
 *****************************************************************************/
