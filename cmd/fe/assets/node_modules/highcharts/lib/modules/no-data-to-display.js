/*
 Highcharts JS v4.1.10 (2015-12-07)
 Plugin for displaying a message when there is no data visible in chart.

 (c) 2010-2014 Highsoft AS
 Author: Oystein Moseng

 License: www.highcharts.com/license
*/
(function(a){typeof module==="object"&&module.exports?module.exports=a:a(Highcharts)})(function(a){function h(){return!!this.points.length}function d(){this.hasData()?this.hideNoData():this.showNoData()}var e=a.seriesTypes,c=a.Chart.prototype,f=a.getOptions(),g=a.extend,i=a.each;g(f.lang,{noData:"No data to display"});f.noData={position:{x:0,y:0,align:"center",verticalAlign:"middle"},attr:{},style:{fontWeight:"bold",fontSize:"12px",color:"#60606a"}};i(["pie","gauge","waterfall","bubble"],function(b){if(e[b])e[b].prototype.hasData=
h});a.Series.prototype.hasData=function(){return this.visible&&this.dataMax!==void 0&&this.dataMin!==void 0};c.showNoData=function(b){var a=this.options,b=b||a.lang.noData,a=a.noData;if(!this.noDataLabel)this.noDataLabel=this.renderer.label(b,0,0,null,null,null,a.useHTML,null,"no-data").attr(a.attr).css(a.style).add(),this.noDataLabel.align(g(this.noDataLabel.getBBox(),a.position),!1,"plotBox")};c.hideNoData=function(){if(this.noDataLabel)this.noDataLabel=this.noDataLabel.destroy()};c.hasData=function(){for(var a=
this.series,c=a.length;c--;)if(a[c].hasData()&&!a[c].options.isInternal)return!0;return!1};c.callbacks.push(function(b){a.addEvent(b,"load",d);a.addEvent(b,"redraw",d)})});
