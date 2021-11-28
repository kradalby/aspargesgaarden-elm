import Swiper from "https://unpkg.com/swiper/swiper-bundle.esm.browser.min.js";
const Swiper = new Swiper(".swiper-container");

// let timeout = 2000;

// function nextSlide() {
//   let activeSlide = document.querySelector(".slide.translate-x-0");
//   if (!activeSlide) {
//     resetSlides();
//     return;
//   }
//   activeSlide.classList.remove("translate-x-0");
//   activeSlide.classList.add("-translate-x-full", "opacity-0");

//   let nextSlide = activeSlide.nextElementSibling;
//   nextSlide.classList.remove("translate-x-full", "opacity-0");
//   nextSlide.classList.add("translate-x-0");
//   activeSlide.classList.remove("-translate-x-full");
// }

// function previousSlide() {
//   let activeSlide = document.querySelector(".slide.translate-x-0");
//   activeSlide.classList.remove("translate-x-0");
//   activeSlide.classList.add("translate-x-full", "opacity-0");

//   let previousSlide = activeSlide.previousElementSibling;
//   previousSlide.classList.remove("-translate-x-full", "opacity-0");
//   previousSlide.classList.add("translate-x-0");
// }

// function resetSlides() {
//   document.querySelectorAll(".slide").forEach((slide, index) => {
//     if (index === 0) {
//       slide.classList.add("translate-x-0");
//       slide.classList.remove(
//         "translate-x-full",
//         "translate-x-full",
//         "opacity-0"
//       );
//     } else {
//       slide.classList.remove("-translate-x-full");
//       slide.classList.add("translate-x-full", "opacity-0");
//     }
//   });
//   nextSlide();
// }

// function setNextTimeout() {
//   nextSlide();
//   setTimeout(function () {
//     setNextTimeout();
//   }, timeout);
// }

// setTimeout(function () {
//   setNextTimeout();
// }, timeout);
