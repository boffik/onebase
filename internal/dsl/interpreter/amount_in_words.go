package interpreter

import (
	"fmt"
	"math"
	"strings"
)

// numberToWords переводит целое неотрицательное число в текст русскими словами.
// gender: 0 — мужской (рубль, тысяча → доллар), 1 — женский (тысяча, копейка).
// Поддерживает диапазон до 999_999_999_999 (~трлн).
//
// Используется в builtin ЧислоПрописью для ПКО/РКО и т.п. (замечание #8).
func numberToWords(n int64, gender int) string {
	if n == 0 {
		return "ноль"
	}
	if n < 0 {
		return "минус " + numberToWords(-n, gender)
	}

	var parts []string
	scales := []struct {
		divisor int64
		one     string
		few     string
		many    string
		gender  int // gender of the count word
	}{
		{1_000_000_000_000, "триллион", "триллиона", "триллионов", 0},
		{1_000_000_000, "миллиард", "миллиарда", "миллиардов", 0},
		{1_000_000, "миллион", "миллиона", "миллионов", 0},
		{1_000, "тысяча", "тысячи", "тысяч", 1},
	}

	for _, sc := range scales {
		if n < sc.divisor {
			continue
		}
		count := n / sc.divisor
		n %= sc.divisor
		parts = append(parts, threeDigitsToWords(count, sc.gender))
		parts = append(parts, pluralRu(count, sc.one, sc.few, sc.many))
	}

	if n > 0 {
		parts = append(parts, threeDigitsToWords(n, gender))
	}
	return strings.Join(parts, " ")
}

// threeDigitsToWords конвертирует число 1..999 в слова.
// gender: 0 — мужской («один», «два»), 1 — женский («одна», «две»).
func threeDigitsToWords(n int64, gender int) string {
	if n == 0 {
		return ""
	}
	hundreds := []string{"", "сто", "двести", "триста", "четыреста", "пятьсот",
		"шестьсот", "семьсот", "восемьсот", "девятьсот"}
	teens := []string{"десять", "одиннадцать", "двенадцать", "тринадцать",
		"четырнадцать", "пятнадцать", "шестнадцать", "семнадцать",
		"восемнадцать", "девятнадцать"}
	tens := []string{"", "", "двадцать", "тридцать", "сорок", "пятьдесят",
		"шестьдесят", "семьдесят", "восемьдесят", "девяносто"}
	unitsMasc := []string{"", "один", "два", "три", "четыре", "пять",
		"шесть", "семь", "восемь", "девять"}
	unitsFem := []string{"", "одна", "две", "три", "четыре", "пять",
		"шесть", "семь", "восемь", "девять"}

	var out []string
	if h := n / 100; h > 0 {
		out = append(out, hundreds[h])
	}
	rem := n % 100
	if rem >= 10 && rem < 20 {
		out = append(out, teens[rem-10])
	} else {
		if t := rem / 10; t > 0 {
			out = append(out, tens[t])
		}
		if u := rem % 10; u > 0 {
			if gender == 1 {
				out = append(out, unitsFem[u])
			} else {
				out = append(out, unitsMasc[u])
			}
		}
	}
	return strings.Join(out, " ")
}

// pluralRu выбирает правильную форму существительного по числительному:
//
//	1 рубль, 2-4 рубля, 5-20 рублей; 21 рубль, 22 рубля, 25 рублей и т.д.
func pluralRu(n int64, one, few, many string) string {
	n = abs64(n) % 100
	if n >= 11 && n <= 19 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// AmountInWords форматирует денежную сумму вида «1234.56» как
// «Одна тысяча двести тридцать четыре рубля 56 копеек».
// currency: "rub" (по умолчанию) | "usd" | "eur" — для будущего расширения,
// сейчас только rub полностью локализован.
func AmountInWords(amount float64, currency string) string {
	// Считаем общее число копеек через ОДНО округление — устойчивее к
	// float-погрешности, чем split-по-точке-плюс-округление-дроби.
	totalKopecks := int64(math.Round(amount * 100))
	negative := totalKopecks < 0
	if negative {
		totalKopecks = -totalKopecks
	}
	rubles := totalKopecks / 100
	kopecks := totalKopecks % 100

	var unitOne, unitFew, unitMany, subOne, subFew, subMany string
	gender := 0
	switch strings.ToLower(currency) {
	case "usd", "$", "доллар":
		unitOne, unitFew, unitMany = "доллар", "доллара", "долларов"
		subOne, subFew, subMany = "цент", "цента", "центов"
	case "eur", "€", "евро":
		unitOne, unitFew, unitMany = "евро", "евро", "евро"
		subOne, subFew, subMany = "цент", "цента", "центов"
	default:
		unitOne, unitFew, unitMany = "рубль", "рубля", "рублей"
		subOne, subFew, subMany = "копейка", "копейки", "копеек"
	}

	words := numberToWords(rubles, gender)
	if negative {
		words = "минус " + words
	}
	// Капитализируем первую букву.
	words = capitalizeFirst(words)
	unit := pluralRu(rubles, unitOne, unitFew, unitMany)
	sub := pluralRu(kopecks, subOne, subFew, subMany)
	return fmt.Sprintf("%s %s %02d %s", words, unit, kopecks, sub)
}

// capitalizeFirst поднимает первый рунный символ в верхний регистр.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	first := r[0]
	switch {
	case first >= 'a' && first <= 'z':
		first -= 32
	case first >= 'а' && first <= 'я':
		first -= 32
	case first == 'ё':
		first = 'Ё'
	}
	r[0] = first
	return string(r)
}
