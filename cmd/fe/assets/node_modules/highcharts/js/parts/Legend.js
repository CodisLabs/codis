/**
 * The overview of the chart's series
 */
var Legend = Highcharts.Legend = function (chart, options) {
	this.init(chart, options);
};

Legend.prototype = {

	/**
	 * Initialize the legend
	 */
	init: function (chart, options) {

		var legend = this,
			itemStyle = options.itemStyle,
			padding,
			itemMarginTop = options.itemMarginTop || 0;

		this.options = options;

		if (!options.enabled) {
			return;
		}

		legend.itemStyle = itemStyle;
		legend.itemHiddenStyle = merge(itemStyle, options.itemHiddenStyle);
		legend.itemMarginTop = itemMarginTop;
		legend.padding = padding = pick(options.padding, 8);
		legend.initialItemX = padding;
		legend.initialItemY = padding - 5; // 5 is the number of pixels above the text
		legend.maxItemWidth = 0;
		legend.chart = chart;
		legend.itemHeight = 0;
		legend.symbolWidth = pick(options.symbolWidth, 16);
		legend.pages = [];


		// Render it
		legend.render();

		// move checkboxes
		addEvent(legend.chart, 'endResize', function () {
			legend.positionCheckboxes();
		});

	},

	/**
	 * Set the colors for the legend item
	 * @param {Object} item A Series or Point instance
	 * @param {Object} visible Dimmed or colored
	 */
	colorizeItem: function (item, visible) {
		var legend = this,
			options = legend.options,
			legendItem = item.legendItem,
			legendLine = item.legendLine,
			legendSymbol = item.legendSymbol,
			hiddenColor = legend.itemHiddenStyle.color,
			textColor = visible ? options.itemStyle.color : hiddenColor,
			symbolColor = visible ? (item.legendColor || item.color || '#CCC') : hiddenColor,
			markerOptions = item.options && item.options.marker,
			symbolAttr = { fill: symbolColor },
			key,
			val;

		if (legendItem) {
			legendItem.css({ fill: textColor, color: textColor }); // color for #1553, oldIE
		}
		if (legendLine) {
			legendLine.attr({ stroke: symbolColor });
		}

		if (legendSymbol) {

			// Apply marker options
			if (markerOptions && legendSymbol.isMarker) { // #585
				symbolAttr.stroke = symbolColor;
				markerOptions = item.convertAttribs(markerOptions);
				for (key in markerOptions) {
					val = markerOptions[key];
					if (val !== UNDEFINED) {
						symbolAttr[key] = val;
					}
				}
			}

			legendSymbol.attr(symbolAttr);
		}
	},

	/**
	 * Position the legend item
	 * @param {Object} item A Series or Point instance
	 */
	positionItem: function (item) {
		var legend = this,
			options = legend.options,
			symbolPadding = options.symbolPadding,
			ltr = !options.rtl,
			legendItemPos = item._legendItemPos,
			itemX = legendItemPos[0],
			itemY = legendItemPos[1],
			checkbox = item.checkbox,
			legendGroup = item.legendGroup;

		if (legendGroup && legendGroup.element) {
			legendGroup.translate(
				ltr ? itemX : legend.legendWidth - itemX - 2 * symbolPadding - 4,
				itemY
			);
		}

		if (checkbox) {
			checkbox.x = itemX;
			checkbox.y = itemY;
		}
	},

	/**
	 * Destroy a single legend item
	 * @param {Object} item The series or point
	 */
	destroyItem: function (item) {
		var checkbox = item.checkbox;

		// destroy SVG elements
		each(['legendItem', 'legendLine', 'legendSymbol', 'legendGroup'], function (key) {
			if (item[key]) {
				item[key] = item[key].destroy();
			}
		});

		if (checkbox) {
			discardElement(item.checkbox);
		}
	},

	/**
	 * Destroys the legend.
	 */
	destroy: function () {
		var legend = this,
			legendGroup = legend.group,
			box = legend.box;

		if (box) {
			legend.box = box.destroy();
		}

		if (legendGroup) {
			legend.group = legendGroup.destroy();
		}
	},

	/**
	 * Position the checkboxes after the width is determined
	 */
	positionCheckboxes: function (scrollOffset) {
		var alignAttr = this.group.alignAttr,
			translateY,
			clipHeight = this.clipHeight || this.legendHeight;

		if (alignAttr) {
			translateY = alignAttr.translateY;
			each(this.allItems, function (item) {
				var checkbox = item.checkbox,
					top;

				if (checkbox) {
					top = (translateY + checkbox.y + (scrollOffset || 0) + 3);
					css(checkbox, {
						left: (alignAttr.translateX + item.checkboxOffset + checkbox.x - 20) + PX,
						top: top + PX,
						display: top > translateY - 6 && top < translateY + clipHeight - 6 ? '' : NONE
					});
				}
			});
		}
	},

	/**
	 * Render the legend title on top of the legend
	 */
	renderTitle: function () {
		var options = this.options,
			padding = this.padding,
			titleOptions = options.title,
			titleHeight = 0,
			bBox;

		if (titleOptions.text) {
			if (!this.title) {
				this.title = this.chart.renderer.label(titleOptions.text, padding - 3, padding - 4, null, null, null, null, null, 'legend-title')
					.attr({ zIndex: 1 })
					.css(titleOptions.style)
					.add(this.group);
			}
			bBox = this.title.getBBox();
			titleHeight = bBox.height;
			this.offsetWidth = bBox.width; // #1717
			this.contentGroup.attr({ translateY: titleHeight });
		}
		this.titleHeight = titleHeight;
	},

	/**
	 * Set the legend item text
	 */
	setText: function (item) {
		var options = this.options;
		item.legendItem.attr({
			text: options.labelFormat ? format(options.labelFormat, item) : options.labelFormatter.call(item)
		});
	},

	/**
	 * Render a single specific legend item
	 * @param {Object} item A series or point
	 */
	renderItem: function (item) {
		var legend = this,
			chart = legend.chart,
			renderer = chart.renderer,
			options = legend.options,
			horizontal = options.layout === 'horizontal',
			symbolWidth = legend.symbolWidth,
			symbolPadding = options.symbolPadding,
			itemStyle = legend.itemStyle,
			itemHiddenStyle = legend.itemHiddenStyle,
			padding = legend.padding,
			itemDistance = horizontal ? pick(options.itemDistance, 20) : 0,
			ltr = !options.rtl,
			itemHeight,
			widthOption = options.width,
			itemMarginBottom = options.itemMarginBottom || 0,
			itemMarginTop = legend.itemMarginTop,
			initialItemX = legend.initialItemX,
			bBox,
			itemWidth,
			li = item.legendItem,
			series = item.series && item.series.drawLegendSymbol ? item.series : item,
			seriesOptions = series.options,
			showCheckbox = legend.createCheckboxForItem && seriesOptions && seriesOptions.showCheckbox,
			useHTML = options.useHTML;

		if (!li) { // generate it once, later move it

			// Generate the group box
			// A group to hold the symbol and text. Text is to be appended in Legend class.
			item.legendGroup = renderer.g('legend-item')
				.attr({ zIndex: 1 })
				.add(legend.scrollGroup);

			// Generate the list item text and add it to the group
			item.legendItem = li = renderer.text(
					'',
					ltr ? symbolWidth + symbolPadding : -symbolPadding,
					legend.baseline || 0,
					useHTML
				)
				.css(merge(item.visible ? itemStyle : itemHiddenStyle)) // merge to prevent modifying original (#1021)
				.attr({
					align: ltr ? 'left' : 'right',
					zIndex: 2
				})
				.add(item.legendGroup);

			// Get the baseline for the first item - the font size is equal for all
			if (!legend.baseline) {
				legend.fontMetrics = renderer.fontMetrics(itemStyle.fontSize, li);
				legend.baseline = legend.fontMetrics.f + 3 + itemMarginTop;
				li.attr('y', legend.baseline);
			}

			// Draw the legend symbol inside the group box
			series.drawLegendSymbol(legend, item);

			if (legend.setItemEvents) {
				legend.setItemEvents(item, li, useHTML, itemStyle, itemHiddenStyle);
			}

			// Colorize the items
			legend.colorizeItem(item, item.visible);

			// add the HTML checkbox on top
			if (showCheckbox) {
				legend.createCheckboxForItem(item);
			}
		}

		// Always update the text
		legend.setText(item);

		// calculate the positions for the next line
		bBox = li.getBBox();

		itemWidth = item.checkboxOffset =
			options.itemWidth ||
			item.legendItemWidth ||
			symbolWidth + symbolPadding + bBox.width + itemDistance + (showCheckbox ? 20 : 0);
		legend.itemHeight = itemHeight = mathRound(item.legendItemHeight || bBox.height);

		// if the item exceeds the width, start a new line
		if (horizontal && legend.itemX - initialItemX + itemWidth >
				(widthOption || (chart.chartWidth - 2 * padding - initialItemX - options.x))) {
			legend.itemX = initialItemX;
			legend.itemY += itemMarginTop + legend.lastLineHeight + itemMarginBottom;
			legend.lastLineHeight = 0; // reset for next line (#915, #3976)
		}

		// If the item exceeds the height, start a new column
		/*if (!horizontal && legend.itemY + options.y + itemHeight > chart.chartHeight - spacingTop - spacingBottom) {
			legend.itemY = legend.initialItemY;
			legend.itemX += legend.maxItemWidth;
			legend.maxItemWidth = 0;
		}*/

		// Set the edge positions
		legend.maxItemWidth = mathMax(legend.maxItemWidth, itemWidth);
		legend.lastItemY = itemMarginTop + legend.itemY + itemMarginBottom;
		legend.lastLineHeight = mathMax(itemHeight, legend.lastLineHeight); // #915

		// cache the position of the newly generated or reordered items
		item._legendItemPos = [legend.itemX, legend.itemY];

		// advance
		if (horizontal) {
			legend.itemX += itemWidth;

		} else {
			legend.itemY += itemMarginTop + itemHeight + itemMarginBottom;
			legend.lastLineHeight = itemHeight;
		}

		// the width of the widest item
		legend.offsetWidth = widthOption || mathMax(
			(horizontal ? legend.itemX - initialItemX - itemDistance : itemWidth) + padding,
			legend.offsetWidth
		);
	},

	/**
	 * Get all items, which is one item per series for normal series and one item per point
	 * for pie series.
	 */
	getAllItems: function () {
		var allItems = [];
		each(this.chart.series, function (series) {
			var seriesOptions = series.options;

			// Handle showInLegend. If the series is linked to another series, defaults to false.
			if (!pick(seriesOptions.showInLegend, !defined(seriesOptions.linkedTo) ? UNDEFINED : false, true)) {
				return;
			}

			// use points or series for the legend item depending on legendType
			allItems = allItems.concat(
					series.legendItems ||
					(seriesOptions.legendType === 'point' ?
							series.data :
							series)
			);
		});
		return allItems;
	},

	/**
	 * Adjust the chart margins by reserving space for the legend on only one side
	 * of the chart. If the position is set to a corner, top or bottom is reserved
	 * for horizontal legends and left or right for vertical ones.
	 */
	adjustMargins: function (margin, spacing) {
		var chart = this.chart,
			options = this.options,
			// Use the first letter of each alignment option in order to detect the side
			alignment = options.align.charAt(0) + options.verticalAlign.charAt(0) + options.layout.charAt(0); // #4189 - use charAt(x) notation instead of [x] for IE7

		if (this.display && !options.floating) {

			each([
				/(lth|ct|rth)/,
				/(rtv|rm|rbv)/,
				/(rbh|cb|lbh)/,
				/(lbv|lm|ltv)/
			], function (alignments, side) {
				if (alignments.test(alignment) && !defined(margin[side])) {
					// Now we have detected on which side of the chart we should reserve space for the legend
					chart[marginNames[side]] = mathMax(
						chart[marginNames[side]],
						chart.legend[(side + 1) % 2 ? 'legendHeight' : 'legendWidth'] +
							[1, -1, -1, 1][side] * options[(side % 2) ? 'x' : 'y'] +
							pick(options.margin, 12) +
							spacing[side]
					);
				}
			});
		}
	},

	/**
	 * Render the legend. This method can be called both before and after
	 * chart.render. If called after, it will only rearrange items instead
	 * of creating new ones.
	 */
	render: function () {
		var legend = this,
			chart = legend.chart,
			renderer = chart.renderer,
			legendGroup = legend.group,
			allItems,
			display,
			legendWidth,
			legendHeight,
			box = legend.box,
			options = legend.options,
			padding = legend.padding,
			legendBorderWidth = options.borderWidth,
			legendBackgroundColor = options.backgroundColor;

		legend.itemX = legend.initialItemX;
		legend.itemY = legend.initialItemY;
		legend.offsetWidth = 0;
		legend.lastItemY = 0;

		if (!legendGroup) {
			legend.group = legendGroup = renderer.g('legend')
				.attr({ zIndex: 7 })
				.add();
			legend.contentGroup = renderer.g()
				.attr({ zIndex: 1 }) // above background
				.add(legendGroup);
			legend.scrollGroup = renderer.g()
				.add(legend.contentGroup);
		}

		legend.renderTitle();

		// add each series or point
		allItems = legend.getAllItems();

		// sort by legendIndex
		stableSort(allItems, function (a, b) {
			return ((a.options && a.options.legendIndex) || 0) - ((b.options && b.options.legendIndex) || 0);
		});

		// reversed legend
		if (options.reversed) {
			allItems.reverse();
		}

		legend.allItems = allItems;
		legend.display = display = !!allItems.length;

		// render the items
		legend.lastLineHeight = 0;
		each(allItems, function (item) {
			legend.renderItem(item);
		});

		// Get the box
		legendWidth = (options.width || legend.offsetWidth) + padding;
		legendHeight = legend.lastItemY + legend.lastLineHeight + legend.titleHeight;
		legendHeight = legend.handleOverflow(legendHeight);
		legendHeight += padding;

		// Draw the border and/or background
		if (legendBorderWidth || legendBackgroundColor) {

			if (!box) {
				legend.box = box = renderer.rect(
					0,
					0,
					legendWidth,
					legendHeight,
					options.borderRadius,
					legendBorderWidth || 0
				).attr({
					stroke: options.borderColor,
					'stroke-width': legendBorderWidth || 0,
					fill: legendBackgroundColor || NONE
				})
				.add(legendGroup)
				.shadow(options.shadow);
				box.isNew = true;

			} else if (legendWidth > 0 && legendHeight > 0) {
				box[box.isNew ? 'attr' : 'animate'](
					box.crisp({ width: legendWidth, height: legendHeight })
				);
				box.isNew = false;
			}

			// hide the border if no items
			box[display ? 'show' : 'hide']();
		}

		legend.legendWidth = legendWidth;
		legend.legendHeight = legendHeight;

		// Now that the legend width and height are established, put the items in the
		// final position
		each(allItems, function (item) {
			legend.positionItem(item);
		});

		// 1.x compatibility: positioning based on style
		/*var props = ['left', 'right', 'top', 'bottom'],
			prop,
			i = 4;
		while (i--) {
			prop = props[i];
			if (options.style[prop] && options.style[prop] !== 'auto') {
				options[i < 2 ? 'align' : 'verticalAlign'] = prop;
				options[i < 2 ? 'x' : 'y'] = pInt(options.style[prop]) * (i % 2 ? -1 : 1);
			}
		}*/

		if (display) {
			legendGroup.align(extend({
				width: legendWidth,
				height: legendHeight
			}, options), true, 'spacingBox');
		}

		if (!chart.isResizing) {
			this.positionCheckboxes();
		}
	},

	/**
	 * Set up the overflow handling by adding navigation with up and down arrows below the
	 * legend.
	 */
	handleOverflow: function (legendHeight) {
		var legend = this,
			chart = this.chart,
			renderer = chart.renderer,
			options = this.options,
			optionsY = options.y,
			alignTop = options.verticalAlign === 'top',
			spaceHeight = chart.spacingBox.height + (alignTop ? -optionsY : optionsY) - this.padding,
			maxHeight = options.maxHeight,
			clipHeight,
			clipRect = this.clipRect,
			navOptions = options.navigation,
			animation = pick(navOptions.animation, true),
			arrowSize = navOptions.arrowSize || 12,
			nav = this.nav,
			pages = this.pages,
			padding = this.padding,
			lastY,
			allItems = this.allItems,
			clipToHeight = function (height) {
				clipRect.attr({
					height: height
				});

				// useHTML
				if (legend.contentGroup.div) {
					legend.contentGroup.div.style.clip = 'rect(' + padding + 'px,9999px,' + (padding + height) + 'px,0)';
				}
			};


		// Adjust the height
		if (options.layout === 'horizontal') {
			spaceHeight /= 2;
		}
		if (maxHeight) {
			spaceHeight = mathMin(spaceHeight, maxHeight);
		}

		// Reset the legend height and adjust the clipping rectangle
		pages.length = 0;
		if (legendHeight > spaceHeight) {

			this.clipHeight = clipHeight = mathMax(spaceHeight - 20 - this.titleHeight - padding, 0);
			this.currentPage = pick(this.currentPage, 1);
			this.fullHeight = legendHeight;

			// Fill pages with Y positions so that the top of each a legend item defines
			// the scroll top for each page (#2098)
			each(allItems, function (item, i) {
				var y = item._legendItemPos[1],
					h = mathRound(item.legendItem.getBBox().height),
					len = pages.length;

				if (!len || (y - pages[len - 1] > clipHeight && (lastY || y) !== pages[len - 1])) {
					pages.push(lastY || y);
					len++;
				}

				if (i === allItems.length - 1 && y + h - pages[len - 1] > clipHeight) {
					pages.push(y);
				}
				if (y !== lastY) {
					lastY = y;
				}
			});

			// Only apply clipping if needed. Clipping causes blurred legend in PDF export (#1787)
			if (!clipRect) {
				clipRect = legend.clipRect = renderer.clipRect(0, padding, 9999, 0);
				legend.contentGroup.clip(clipRect);
			}

			clipToHeight(clipHeight);

			// Add navigation elements
			if (!nav) {
				this.nav = nav = renderer.g().attr({ zIndex: 1 }).add(this.group);
				this.up = renderer.symbol('triangle', 0, 0, arrowSize, arrowSize)
					.on('click', function () {
						legend.scroll(-1, animation);
					})
					.add(nav);
				this.pager = renderer.text('', 15, 10)
					.css(navOptions.style)
					.add(nav);
				this.down = renderer.symbol('triangle-down', 0, 0, arrowSize, arrowSize)
					.on('click', function () {
						legend.scroll(1, animation);
					})
					.add(nav);
			}

			// Set initial position
			legend.scroll(0);

			legendHeight = spaceHeight;

		} else if (nav) {
			clipToHeight(chart.chartHeight);
			nav.hide();
			this.scrollGroup.attr({
				translateY: 1
			});
			this.clipHeight = 0; // #1379
		}

		return legendHeight;
	},

	/**
	 * Scroll the legend by a number of pages
	 * @param {Object} scrollBy
	 * @param {Object} animation
	 */
	scroll: function (scrollBy, animation) {
		var pages = this.pages,
			pageCount = pages.length,
			currentPage = this.currentPage + scrollBy,
			clipHeight = this.clipHeight,
			navOptions = this.options.navigation,
			activeColor = navOptions.activeColor,
			inactiveColor = navOptions.inactiveColor,
			pager = this.pager,
			padding = this.padding,
			scrollOffset;

		// When resizing while looking at the last page
		if (currentPage > pageCount) {
			currentPage = pageCount;
		}

		if (currentPage > 0) {

			if (animation !== UNDEFINED) {
				setAnimation(animation, this.chart);
			}

			this.nav.attr({
				translateX: padding,
				translateY: clipHeight + this.padding + 7 + this.titleHeight,
				visibility: VISIBLE
			});
			this.up.attr({
					fill: currentPage === 1 ? inactiveColor : activeColor
				})
				.css({
					cursor: currentPage === 1 ? 'default' : 'pointer'
				});
			pager.attr({
				text: currentPage + '/' + pageCount
			});
			this.down.attr({
					x: 18 + this.pager.getBBox().width, // adjust to text width
					fill: currentPage === pageCount ? inactiveColor : activeColor
				})
				.css({
					cursor: currentPage === pageCount ? 'default' : 'pointer'
				});

			scrollOffset = -pages[currentPage - 1] + this.initialItemY;

			this.scrollGroup.animate({
				translateY: scrollOffset
			});

			this.currentPage = currentPage;
			this.positionCheckboxes(scrollOffset);
		}

	}

};

/*
 * LegendSymbolMixin
 */

var LegendSymbolMixin = Highcharts.LegendSymbolMixin = {

	/**
	 * Get the series' symbol in the legend
	 *
	 * @param {Object} legend The legend object
	 * @param {Object} item The series (this) or point
	 */
	drawRectangle: function (legend, item) {
		var symbolHeight = legend.options.symbolHeight || legend.fontMetrics.f;

		item.legendSymbol = this.chart.renderer.rect(
			0,
			legend.baseline - symbolHeight + 1, // #3988
			legend.symbolWidth,
			symbolHeight,
			legend.options.symbolRadius || 0
		).attr({
			zIndex: 3
		}).add(item.legendGroup);

	},

	/**
	 * Get the series' symbol in the legend. This method should be overridable to create custom
	 * symbols through Highcharts.seriesTypes[type].prototype.drawLegendSymbols.
	 *
	 * @param {Object} legend The legend object
	 */
	drawLineMarker: function (legend) {

		var options = this.options,
			markerOptions = options.marker,
			radius,
			legendSymbol,
			symbolWidth = legend.symbolWidth,
			renderer = this.chart.renderer,
			legendItemGroup = this.legendGroup,
			verticalCenter = legend.baseline - mathRound(legend.fontMetrics.b * 0.3),
			attr;

		// Draw the line
		if (options.lineWidth) {
			attr = {
				'stroke-width': options.lineWidth
			};
			if (options.dashStyle) {
				attr.dashstyle = options.dashStyle;
			}
			this.legendLine = renderer.path([
				M,
				0,
				verticalCenter,
				L,
				symbolWidth,
				verticalCenter
			])
			.attr(attr)
			.add(legendItemGroup);
		}

		// Draw the marker
		if (markerOptions && markerOptions.enabled !== false) {
			radius = markerOptions.radius;
			this.legendSymbol = legendSymbol = renderer.symbol(
				this.symbol,
				(symbolWidth / 2) - radius,
				verticalCenter - radius,
				2 * radius,
				2 * radius
			)
			.add(legendItemGroup);
			legendSymbol.isMarker = true;
		}
	}
};

// Workaround for #2030, horizontal legend items not displaying in IE11 Preview,
// and for #2580, a similar drawing flaw in Firefox 26.
// Explore if there's a general cause for this. The problem may be related
// to nested group elements, as the legend item texts are within 4 group elements.
if (/Trident\/7\.0/.test(userAgent) || isFirefox) {
	wrap(Legend.prototype, 'positionItem', function (proceed, item) {
		var legend = this,
			runPositionItem = function () { // If chart destroyed in sync, this is undefined (#2030)
				if (item._legendItemPos) {
					proceed.call(legend, item);
				}
			};

		// Do it now, for export and to get checkbox placement
		runPositionItem();

		// Do it after to work around the core issue
		setTimeout(runPositionItem);
	});
}
